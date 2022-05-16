package erunner

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ibalajiarun/go-consensus/erunner/config"
	"github.com/ibalajiarun/go-consensus/erunner/discovery"
	"github.com/ibalajiarun/go-consensus/erunner/kube"
	"github.com/ibalajiarun/go-consensus/erunner/promquery"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PROMHOST = "http://localhost:9090/"
)

var (
	ErrThroughputZero = errors.New("throughput zero")
)

const (
	promQueryMem = `Min(node_memory_MemFree_bytes{job="node-exporter"})/(1024.0*1024*1024)`
)

type DataPoint struct {
	Throughput float64
	Latency    float64
}

func (r *Runner) getMinFreeNodeMemory() (float64, error) {
	memory, err := promquery.Query(PROMHOST, time.Now(), promQueryMem)
	if err != nil {
		return 0, fmt.Errorf("error querying throughput stats %w", err)
	}

	num, err := strconv.ParseFloat(memory[0].Values["0"], 64)
	if err != nil {
		panic(err)
	}
	return num, nil
}

func (r *Runner) runDatapointExp(
	ctx context.Context,
	dirPrefix string,
	infPlan config.PerInflightPlan,
	runConfig discovery.Config,
	startAt int,
) ([]*DataPoint, error) {
	r.logger.Infof("Running Data Point Experiment: Inflight Plan: %v; Start At %v", infPlan, startAt)

	// Check if cluster is active
	kCli, err := kube.NewManager(r.plan.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to cluster: %w", err)
	}

	// Get Node Information
	nodes, err := kCli.GetAllNodeDetails()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve node details: %w", err)
	}
	nodeIpMapping := make(map[string]string)
	for _, item := range nodes.Items {
		var externalIP, internalIP string
		for _, addr := range item.Status.Addresses {
			switch addr.Type {
			case corev1.NodeExternalIP:
				externalIP = addr.Address
			case corev1.NodeInternalIP:
				internalIP = addr.Address
			}
		}
		if len(externalIP) == 0 || len(internalIP) == 0 {
			return nil, fmt.Errorf("cannot find external IP for node %v", item.Name)
		}
		nodeIpMapping[fmt.Sprintf("%s/%s", item.Name, internalIP)] = externalIP
	}

	// Start Service Discovery
	lis, err := net.Listen("tcp", ":7000")
	if err != nil {
		return nil, fmt.Errorf("unable to listen on socket: %w", err)
	}

	d := discovery.NewDiscoveryServer(r.logger, runConfig, nodeIpMapping)
	go d.Serve(lis)
	// Stop Service Discovery
	defer d.Stop()

	// Apply Kube Config
	for _, specKind := range r.specs {
		switch specKind.Kind {
		case "ReplicaSet":
			spec, ok := specKind.Spec.(*appsv1.ReplicaSet)
			if !ok {
				return nil, fmt.Errorf("unable to cast type to ReplicaSet: %v", specKind.Spec)
			}
			switch spec.Name {
			case "go-consensus-server":
				spec.Spec.Replicas = &runConfig.PeerCount
			case "go-consensus-client":
				spec.Spec.Replicas = &runConfig.ClientCount
			default:
				return nil, fmt.Errorf("invalid spec name to assign counts: %v", spec)
			}

			result, err := kCli.CreateReplicaSet(v1.NamespaceDefault, spec)
			if err != nil {
				return nil, fmt.Errorf("unable to create ReplicaSet %v: %w", spec, err)
			}
			// Delete when done
			defer func() {
				r.logger.Infof("Deleting RS: %v...", result.Name)
				err = kCli.DeleteReplicaSet(result.Namespace, result.Name)
				if err != nil {
					r.logger.Infof("Force deleting pods: %v...", result.Name)
					kCli.ForceDeletePods(result.Namespace, fmt.Sprintf("app=%s", result.Name))
					time.Sleep(15 * time.Second)
				}
			}()
		case "Service":
			spec, ok := specKind.Spec.(*corev1.Service)
			if !ok {
				return nil, fmt.Errorf("unable to cast type to ReplicaSet: %v", specKind.Spec)
			}
			result, err := kCli.CreateService(v1.NamespaceDefault, spec)
			if err != nil {
				return nil, fmt.Errorf("unable to create ReplicaSet %v: %w", spec, err)
			}
			// Delete when done
			defer func() {
				_ = kCli.DeleteService(result.Namespace, result.Name)
			}()
		}
	}

	wCtx, wCancel := context.WithTimeout(ctx, 5*time.Minute)

	r.logger.Info("Waiting for service discovery to notify...")
	// Wait for Service Discovery to notify
	werr := d.WaitReady(wCtx)
	wCancel()
	if werr != nil {
		r.logger.Errorf("discovery service returned an error: %w", werr)
		return nil, fmt.Errorf("discovery service returned an error: %w", werr)
	}

	r.logger.Info("Triggering clients")
	// Notify Service Discovery to inform clients
	d.TriggerClients()

	dps := make([]*DataPoint, 0, len(infPlan.ClientCounts))
	scale := false
	for i := startAt; i < len(infPlan.ClientCounts); i++ {
		count := infPlan.ClientCounts[i]

		select {
		case <-r.skipBatchC:
			r.logger.Infof("skipping current Algorithm: %s", dirPrefix)
			return dps, nil
		default:
		}

		r.logger.Infof("Scaling clients to %v...", count)
		err := kCli.ScaleReplicaSet(v1.NamespaceDefault, "go-consensus-client", count)
		if err != nil {
			return dps, fmt.Errorf("unable to scale to %v: %w", count, err)
		}
		dp, err := r.doOneRun(ctx, dirPrefix, kCli, runConfig.ClientConfig.MaxInflightRequests, count, scale)
		if err != nil {
			if errors.Is(err, ErrThroughputZero) {
				r.logger.Info("Skipping because throughput is zero")
				return dps, err
			}
			return dps, err
		}
		dps = append(dps, dp)

		freeMem, err := r.getMinFreeNodeMemory()
		r.logger.Infof("average free memory: %v", freeMem)
		if err != nil {
			return dps, fmt.Errorf("error accessing memory info: %w", err)
		} else if freeMem < 8 {
			r.logger.Infof("resetting deployment due to low free memory: %v", freeMem)
			return dps, nil
		}

		scale = true
	}

	return dps, nil
}

func (r *Runner) doOneRun(
	ctx context.Context,
	dirPrefix string,
	kCli *kube.KubeManager,
	inf uint32,
	cnt int32,
	scale bool,
) (*DataPoint, error) {
	r.logger.Infof("Running experiment with %v inflight requests and %v clients", inf, cnt)

	waitDuration := r.plan.DatapointTime.Duration
	if scale {
		waitDuration = r.plan.ScaleDatapointTime.Duration
	}

	r.logger.Infof("Waiting for %v", waitDuration)

	// Wait for some time
	// Monitor for pod crashes and terminate experiment
	mCtx, mCancel := context.WithTimeout(ctx, waitDuration)
	err := kCli.MonitorReplicaSet(mCtx, v1.NamespaceDefault, "app=go-consensus-server")
	mCancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("scraping experiment due to signal: %w", err)
		}
		return nil, fmt.Errorf("scraping experiment becauase pod crashed: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("cancelling due to signal: %w", ctx.Err())
	default:
	}

	// Gather time series from Prometheus (for past one minute?)
	startTime := time.Now().Add(-30 * time.Second)
	endTime := time.Now()
	tpsResults, err := promquery.QueryRange(PROMHOST, startTime, endTime, "sum(rate(goconsensus_client_request_latency_count[30s]))")
	if err != nil {
		return nil, fmt.Errorf("error querying throughput stats %w", err)
	}
	latResults, err := promquery.QueryRange(PROMHOST, startTime, endTime, "avg(rate(goconsensus_client_request_latency_sum[30s]))/avg(rate(goconsensus_client_request_latency_count[30s]))")
	if err != nil {
		return nil, fmt.Errorf("error querying latency stats %w", err)
	}

	// Write time series to file
	file, err := os.Create(fmt.Sprintf("%s/datapoint-%d-%d.csv", dirPrefix, inf, cnt))
	if err != nil {
		return nil, fmt.Errorf("error creating file to write data: %w", err)
	}
	defer file.Close()
	promquery.CsvWriter(file, tpsResults, latResults)

	plt1, err := plotTimeSeries("Latency", latResults)
	if err != nil {
		return nil, fmt.Errorf("plot error: %w", err)
	}
	if err := plt1.Save(4*vg.Inch, 4*vg.Inch, fmt.Sprintf("%s/lat-%d-%d.png", dirPrefix, inf, cnt)); err != nil {
		return nil, fmt.Errorf("unable to save plot: %w", err)
	}

	plt2, err := plotTimeSeries("Throughput", tpsResults)
	if err != nil {
		return nil, fmt.Errorf("plot error: %w", err)
	}
	if err := plt2.Save(4*vg.Inch, 4*vg.Inch, fmt.Sprintf("%s/tps-%d-%d.png", dirPrefix, inf, cnt)); err != nil {
		return nil, fmt.Errorf("unable to save plot: %w", err)
	}

	// Generate data point
	avgTps := averageResults(tpsResults)
	avgLat := averageResults(latResults)

	r.logger.Infof("DataPoint experiment finished. Lat: %f, Tps: %f", avgLat, avgTps)

	// Return data point
	dp := &DataPoint{
		Throughput: avgTps,
		Latency:    avgLat,
	}

	if avgTps == 0 {
		return dp, ErrThroughputZero
	}

	return dp, nil
}

func averageResults(results []promquery.Result) float64 {
	sum := float64(0)
	count := 0
	for _, result := range results {
		for _, val := range result.Values {
			num, err := strconv.ParseFloat(val, 64)
			if err != nil {
				panic(err)
			}
			sum += num
			count++
		}
	}
	return sum / float64(count)
}

func plotTimeSeries(title string, results []promquery.Result) (*plot.Plot, error) {
	p, err := plot.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create plot: %w", err)
	}

	p.Title.Text = title
	p.X.Label.Text = "Time"
	p.Y.Min = 0

	for _, res := range results {
		pts := make(plotter.XYs, len(res.Values))
		idx := 0
		for _, pt := range res.Values {
			pts[idx].X = float64(idx)

			y, err := strconv.ParseFloat(pt, 64)
			if err != nil {
				return nil, err
			}
			if math.IsNaN(y) {
				y = 0
			}
			pts[idx].Y = y
			idx++
		}
		err := plotutil.AddLinePoints(p, res.Metric, pts)
		if err != nil {
			return nil, fmt.Errorf("unable to add points to plot: %w", err)
		}
	}

	// Save the plot to a PNG file.
	return p, nil
}
