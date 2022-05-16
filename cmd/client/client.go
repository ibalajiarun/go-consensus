package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ibalajiarun/go-consensus/cmd/client/connection"
	"github.com/ibalajiarun/go-consensus/cmd/client/worker"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	transport "github.com/ibalajiarun/go-consensus/transport"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var (
	help  = flag.Bool("help", false, "")
	maddr = flag.String("master", "ssrg:5060", "master address")
)

var (
	latHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "client",
		Name:      "request_latency",
		Help:      "Client-side Request Latency",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 16),
	})
	promActivePeerGuage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "goconsensus",
		Subsystem: "client",
		Name:      "grpc_active_peers",
		Help:      "Active peer connections",
	})
)

func startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":9091", nil)
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.CommandLine.MarkHidden("help")
	flag.Parse()

	// Start a Datadog tracer, optionally providing a set of options,
	// returning an opentracing.Tracer which wraps it.
	t := opentracer.New(tracer.WithServiceName("go-consensus"))
	defer tracer.Stop() // important for data integrity (flushes any leftovers)

	// Use it with the Opentracing API. The (already started) Datadog tracer
	// may be used in parallel with the Opentracing API if desired.
	opentracing.SetGlobalTracer(t)

	logger := logger.NewDefaultLogger()

	if *help {
		logger.Errorf("Usage of %s:\n", os.Args[0])
		logger.Fatal(flag.CommandLine.FlagUsagesWrapped(120))
		return
	}

	go startMetricsServer()

	res, localIDs, err := getConfigFromMaster(*maddr)
	if err != nil {
		panic(err)
	}
	config := res.Config
	appID := res.ClientId
	leaderID := int(config.LeaderID)

	logger.Infof("AppID %v\n", appID)
	logger.Infof("Local IDs %v\n", localIDs)
	logger.Infof("Leader ID %v\n", leaderID)
	logger.Infof("Batch size %v\n", config.RequestOpsBatchSize)

	if res.Config.LogVerbose {
		logger.EnableDebug()
	}

	time.Sleep(time.Duration(config.SleepTimeSeconds) * time.Second)

	logger.Info("Connecting to peers...")
	var mu sync.Mutex
	peers := make(map[peerpb.PeerID]*transport.ExternalClient, len(config.Nodes))
	g, ctx := errgroup.WithContext(context.Background())
	for _, nInfo := range config.Nodes {
		nInfo := nInfo
		g.Go(func() error {
			addrStr := fmt.Sprintf("%s:7000", nInfo.PodIP)
		retry_connect:
			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			c, err := transport.NewExternalClient(ctx, addrStr)
			cancel()
			if err != nil {
				switch err {
				case context.DeadlineExceeded:
					logger.Errorf("Timedout connecting to peer %v (%v): %v. Trying again.", nInfo.PeerID, nInfo, err)
					goto retry_connect
				default:
					logger.Fatalf("Unable to connect to peer %v: %e", nInfo.PeerID, err)
				}
				return err
			}
			promActivePeerGuage.Inc()
			mu.Lock()
			peers[nInfo.PeerID] = c
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		logger.Fatalf("Unable to connect to peer... %e", err)
	}
	logger.Infof("Connected to %v peers.", len(peers))

	metricC := make(chan float64, int(config.MaxInflightRequests)*2)
	var wg sync.WaitGroup
	doneC := make(chan bool)

	clients := make([]*worker.ClientWorker, config.MaxInflightRequests)
	var nextIDPtr uint64
	for i := 0; i < int(config.MaxInflightRequests); i++ {
		conn := connection.New(peers, logger)
		conn.Run()

		client := worker.NewClientWorker(conn, logger, config, appID,
			uint32(i), localIDs, metricC, &nextIDPtr)

		var reqFunc func(*commandpb.Command) (*commandpb.CommandResult, error)
		switch config.Algorithm {
		case peerpb.Algorithm_LinHybster:
			fallthrough
		case peerpb.Algorithm_SBFT:
			fallthrough
		case peerpb.Algorithm_SBFTSlow:
			reqFunc = client.SBFT
		case peerpb.Algorithm_SBFTx:
			fallthrough
		case peerpb.Algorithm_DQSBFTSlow:
			fallthrough
		case peerpb.Algorithm_Destiny:
			reqFunc = client.ThresholdMultiPrimary
		case peerpb.Algorithm_Dester:
			reqFunc = client.Dester
		case peerpb.Algorithm_DQPBFT:
			fallthrough
		case peerpb.Algorithm_Hybsterx:
			reqFunc = client.MultiPrimary
		case peerpb.Algorithm_Hotstuff:
			fallthrough
		case peerpb.Algorithm_ChainHotstuff:
			fallthrough
		case peerpb.Algorithm_MirBFT:
			fallthrough
		case peerpb.Algorithm_RCC:
			fallthrough
		case peerpb.Algorithm_MultiChainDuoBFTRCC:
			fallthrough
		case peerpb.Algorithm_Dispel:
			reqFunc = client.MultiPrimaryRRTarget
		case peerpb.Algorithm_Prime:
			reqFunc = client.Prime
		default:
			reqFunc = client.Generic
		}

		client.Init(reqFunc)
		clients[i] = client
	}

	for i := 0; i < int(config.MaxInflightRequests); i++ {
		wg.Add(1)
		go func(id uint64, client *worker.ClientWorker) {
			client.Run()
			wg.Done()
		}(uint64(i), clients[i])
	}

	go func() {
		for {
			select {
			case lat := <-metricC:
				latHist.Observe(float64(lat))
				logger.Debug(lat)
			case <-doneC:
				return
			}
		}
	}()
	wg.Wait()
	doneC <- true
}
