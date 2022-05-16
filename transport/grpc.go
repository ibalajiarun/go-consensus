package transport

import (
	"context"
	"io"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	transpb "github.com/ibalajiarun/go-consensus/transport/transportpb"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
)

var (
	promActivePeerGuage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "grpc_active_peers",
		Help:      "Active peer connections",
	})
)

type grpcTransport struct {
	id      peerpb.PeerID
	addr    string
	peers   map[peerpb.PeerID]string
	handler MessageHandler

	grpc       *grpc.Server
	dialCtx    context.Context
	dialCancel func()
	clientMu   sync.Mutex
	clientBufs map[peerpb.PeerID]chan<- *transpb.TransMsg
	sortBuf    byDestination
}

// NewGRPCTransport creates a new Transport that uses gRPC streams.
func NewGRPCTransport() Transport {
	return new(grpcTransport)
}

func (g *grpcTransport) Init(id peerpb.PeerID, addr string, peers map[peerpb.PeerID]string) {
	g.id = id
	g.addr = addr
	g.peers = peers
	g.clientBufs = make(map[peerpb.PeerID]chan<- *transpb.TransMsg)
	g.dialCtx, g.dialCancel = context.WithCancel(context.Background())
	g.grpc = grpc.NewServer(grpc.UnaryInterceptor(
		otgrpc.OpenTracingServerInterceptor(
			opentracing.GlobalTracer())),
		grpc.StreamInterceptor(
			otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())))
	transpb.RegisterPeerTransportServer(g.grpc, g)
	transpb.RegisterCommandServiceServer(g.grpc, g)
}

func (g *grpcTransport) Serve(h MessageHandler) {
	g.handler = h

	var lis net.Listener
	for i := 0; ; i++ {
		var err error
		lis, err = net.Listen("tcp", g.addr)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "address already in use") {
			if i > 16 {
				log.Printf("waiting to listen %v", err)
			}
			continue
		}
		log.Fatal(err)
	}

	log.Printf("Listening on %v\n", g.addr)
	if err := g.grpc.Serve(lis); err != nil {
		switch err {
		case grpc.ErrServerStopped:
		default:
			log.Fatal(err)
		}
	}
}

func (g *grpcTransport) Message(stream transpb.PeerTransport_MessageServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		g.handler.HandleMessage(in)
	}
}

func (g *grpcTransport) Apply(
	ctx context.Context,
	pkt *transpb.ClientPacket,
) (*transpb.ClientPacket, error) {
	return g.handler.HandleCommand(ctx, pkt)
}

func (g *grpcTransport) Send(msgs []peerpb.Message) {
	// Group messages by destination and combine.
	g.sortMsgs(msgs)
	st := 0
	to := msgs[0].To
	for i := 1; i < len(msgs); i++ {
		if msgs[i].To != to {
			g.sendAsync(peerpb.PeerID(to), &transpb.TransMsg{
				Msgs: msgs[st:i],
			})
			to = msgs[i].To
			st = i
		}
	}
	g.sendAsync(peerpb.PeerID(to), &transpb.TransMsg{
		Msgs: msgs[st:],
	})
}

func (g *grpcTransport) sortMsgs(msgs []peerpb.Message) {
	g.sortBuf = msgs
	sort.Stable(&g.sortBuf)
}

// byDestination implements sort.Interface.
type byDestination []peerpb.Message

func (s byDestination) Len() int {
	return len(s)
}

func (s byDestination) Less(i, j int) bool {
	return s[i].To < s[j].To
}

func (s byDestination) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (g *grpcTransport) sendAsync(to peerpb.PeerID, m *transpb.TransMsg) {
	if to == g.id {
		g.handler.HandleMessage(m)
		return
	}

	g.clientMu.Lock()
	buf, ok := g.clientBufs[to]
	g.clientMu.Unlock()
	if ok {
		select {
		case buf <- m:
		case <-g.dialCtx.Done():
		}
		return
	}

	g.clientMu.Lock()
	defer g.clientMu.Unlock()

	c := make(chan *transpb.TransMsg, 1024)
	g.clientBufs[to] = c
	promActivePeerGuage.Inc()
	go g.sender(to, c)

	//TODO: It's a patch. clean up.
	select {
	case c <- m:
	case <-g.dialCtx.Done():
	}
}

func (g *grpcTransport) sender(to peerpb.PeerID, c <-chan *transpb.TransMsg) {
	defer func() {
		g.clientMu.Lock()
		defer g.clientMu.Unlock()
		delete(g.clientBufs, to)
		promActivePeerGuage.Dec()
	}()

	url, ok := g.peers[to]
	if !ok {
		log.Fatalf("unknown peer %d", to)
	}

dial:
	log.Printf("Dialing to %d: %v\n", to, url)
	ctx, cancel := context.WithTimeout(g.dialCtx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, url,
		grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.5,
				Jitter:     0.2,
				MaxDelay:   5 * time.Second,
			},
		}),
		grpc.WithInitialWindowSize(1<<20),
	)
	if err != nil {
		switch err {
		case context.Canceled:
		case context.DeadlineExceeded:
			log.Printf("error when dialing %d: %v. trying again...", to, err)
			goto dial
		default:
			log.Printf("error when dialing %d: %v", to, err)
		}
		return
	}
	log.Printf("Connection established to %v: %v\n", to, url)

	defer conn.Close()

	client := transpb.NewPeerTransportClient(conn)
	stream, err := client.Message(g.dialCtx)
	if err != nil {
		switch err {
		case context.Canceled:
			return
		default:
			log.Fatal(err)
		}
	}
	for m := range c {
		if err := stream.Send(m); err != nil {
			switch err {
			case context.Canceled:
				return
			case io.EOF:
				return
			default:
				log.Fatal(err)
			}
		}
	}
}

func (g *grpcTransport) Close() {
	g.grpc.Stop()
	g.dialCancel()
	g.clientMu.Lock()
	defer g.clientMu.Unlock()
	for id, c := range g.clientBufs {
		close(c)
		delete(g.clientBufs, id)
	}
}
