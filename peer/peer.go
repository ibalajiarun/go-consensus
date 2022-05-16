package peer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	pb "github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/transport"
	transpb "github.com/ibalajiarun/go-consensus/transport/transportpb"
	"github.com/ibalajiarun/go-consensus/utils/helper"
	"github.com/ibalajiarun/go-consensus/utils/signer"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
)

type Ready struct {
	Messages        []pb.Message
	OrderedCommands []ExecPacket
}

func (rd Ready) containsUpdates() bool {
	return len(rd.Messages) > 0 || len(rd.OrderedCommands) > 0
}

// LocalConfig contains configurations for constructing a Peer.
type LocalConfig struct {
	*pb.PeerConfig
	Peers      []pb.PeerID
	PeerAddrs  map[pb.PeerID]string
	ListenAddr string
	ID         pb.PeerID
	Logger     logger.Logger
	RandSeed   int64
}

var (
	promBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "command_batch_size",
		Help:      "Command Batch Size",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10),
	})
	promMsgCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "message_flush_count",
		Help:      "Message Flush Count",
		Buckets:   prometheus.LinearBuckets(10, 10, 20),
	})
	promReqQSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "request_queue_size",
		Help:      "Request Queue Size",
		Buckets:   prometheus.LinearBuckets(10, 10, 20),
	})
	promMsgQSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "msg_queue_size",
		Help:      "Message Queue Size",
		Buckets:   prometheus.LinearBuckets(10, 10, 20),
	})
	promSendMsgSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "send_msgs_count",
		Help:      "Send Messages Size",
		Buckets:   prometheus.LinearBuckets(10, 10, 20),
	})
)

// Peer is a member of a Raft consensus group. Its primary roles are to:
// 1. route incoming Raft messages
// 2. periodically tick the Raft RawNode
// 3. serve as a scheduler for Raft proposal pipeline events
type Peer struct {
	id pb.PeerID

	mu   sync.Mutex
	sig  chan struct{} // signaled to wake-up Raft loop
	done int32
	wg   sync.WaitGroup

	logger logger.Logger
	cfg    *LocalConfig
	n      Protocol
	t      transport.Transport
	pl     asyncApplier

	rb reqBuf
	ct command.Tracker

	msgs        chan *transpb.TransMsg
	flushCmdsFn func([]reqBufElem)

	callbackC chan ExecCallback

	signer *signer.Signer
}

// New creates a new Peer.
func New(
	cfg *LocalConfig,
	t transport.Transport,
	n Protocol,
) *Peer {
	p := &Peer{
		id: cfg.ID,

		sig: make(chan struct{}, 1),

		logger: cfg.Logger,
		cfg:    cfg,
		n:      n,
		t:      t,
		pl:     newAsyncApplier(),
		ct:     command.MakeTracker(cfg.Logger),
		msgs:   make(chan *transpb.TransMsg, 1024),

		signer: signer.NewSigner(),
	}
	p.t.Init(cfg.ID, cfg.ListenAddr, cfg.PeerAddrs)
	p.pl.Init(cfg.Logger, &p.mu, p.n, p.t, &p.ct, p.sig)
	p.rb.init(cfg.ReqBufThreshold)
	p.flushCmdsFn = p.flushCmds
	p.initTimers()
	go p.t.Serve(p)
	return p
}

// Run starts the Peer's processing loop.
func (p *Peer) Run() {
	p.wg.Add(2)
	p.pl.Start()
	go p.ticker()
	defer p.wg.Done()

	for {
		<-p.sig
		if p.stopped() {
			p.mu.Lock()
			p.rb.flush(p.flushCmdsFn, true)
			p.mu.Unlock()
			return
		}
		p.mu.Lock()
		p.flushMsgs()
		p.rb.flush(p.flushCmdsFn, false)
		p.pl.RunOnce()
		p.mu.Unlock()
	}
}

func (p *Peer) signal() {
	select {
	case p.sig <- struct{}{}:
	default:
		// Already signaled.
	}
}

func (p *Peer) initTimers() {
}

func (p *Peer) ticker() {
	defer p.wg.Done()
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for !p.stopped() {
		<-t.C
		p.mu.Lock()
		p.n.Tick()
		p.mu.Unlock()
		p.signal()
	}
}

// Stop stops all processing and releases all resources held by Peer.
func (p *Peer) Stop() {
	atomic.StoreInt32(&p.done, 1)
	p.signal()
	p.t.Close()
	p.wg.Wait()
	p.ct.FinishAll()
	p.pl.Stop()
}

func (p *Peer) stopped() bool {
	return atomic.LoadInt32(&p.done) == 1
}

// HandleMessage implements transport.MessageHandler.
func (p *Peer) HandleMessage(m *transpb.TransMsg) {
	p.msgs <- m
	p.signal()
}

func (p *Peer) HandleCommand(
	ctx context.Context,
	pkt *transpb.ClientPacket,
) (*transpb.ClientPacket, error) {
	cmd := &commandpb.Command{}

	err := proto.Unmarshal(pkt.Message, cmd)
	if err != nil {
		p.logger.Panicf("Unable to unmarshal msg %v: %v", pkt.Message, err)
	}

	if cmd.Target == p.id {
		if p.signer.Verify(pkt.Message, pkt.Signature) != nil {
			p.logger.Panicf("Unable to verify signature: %v", pkt)
		}
	}

	p.logger.Debugf("Received command: %v", cmd)
	sp, ctx := opentracing.StartSpanFromContext(ctx, "handle_command")
	defer sp.Finish()

	buf := make(opentracing.TextMapCarrier)
	if err := opentracing.GlobalTracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapWriter(buf)); err != nil {
		p.logger.Errorf("Error injecting trace handle_command: %w", err)
		p.logger.Panicf("Error injecting trace handle_command: %w", err)
	}
	cmd.TraceInfo = buf

	c := make(chan *commandpb.CommandResult, 1)
	el := reqBufElem{cmd, c}

	p.rb.add(el)
	p.signal()
	if p.stopped() {
		close(c)
		return nil, grpc.ErrServerStopped
	}

	select {
	case res := <-c:
		p.logger.Debugf("Replying for command: %v", res)
		mBytes, sign := helper.MarshallAndSign(res, p.signer)
		cp := &transpb.ClientPacket{
			Message:   mBytes,
			Signature: sign,
		}
		return cp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Peer) flushCmds(rb []reqBufElem) {
	if len(rb) == 0 {
		return
	}

	promReqQSize.Observe(float64(len(rb)))
	for i := range rb {
		cmd := rb[i].cmd
		if ok := p.ct.Register(cmd, rb[i].c); !ok {
			continue
		}
		if cmd.Target == p.id {
			p.n.Request(cmd)
		}
	}
}

func (p *Peer) flushMsgs() {
	p.n.AsyncCallback()

	count := 0
	times := 0
	size := len(p.msgs)
	for {
		select {
		case m := <-p.msgs:
			count = count + len(m.Msgs)
			for i := range m.Msgs {
				p.n.Step(m.Msgs[i])
			}
		default:
			if count > 0 {
				promMsgCount.Observe(float64(count))
			}
			promMsgQSize.Observe(float64(size))
			return
		}
		times++
		if times >= len(p.cfg.Peers) {
			return
		}
	}
}
