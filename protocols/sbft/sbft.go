package sbft

import (
	"context"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/threshsign2"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"
	"github.com/opentracing/opentracing-go"
)

type sbft struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	log     map[pb.Index]*instance
	view    pb.View
	index   pb.Index
	execIdx pb.Index

	active bool

	msgs              []peerpb.Message
	committedCommands []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	signer *threshsign2.ThreshsignEnclave

	f int
	c int

	executedCmds []peer.ExecPacket
	rQuorum      *rquorum

	signDispatcher   *worker.Dispatcher
	aggDispatcher    *worker.Dispatcher
	verifyDispatcher *worker.Dispatcher
	callbackC        chan func()
}

func NewSBFT(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Index]*instance, 1024*100)
	f := (len(c.Peers) - 2*int(c.MaxFastFailures) - 1) / 3
	if f < 1 {
		c.Logger.Panicf("Invalid failure count: %v", f)
	}
	if f != int(c.MaxFailures) {
		c.Logger.Panicf("Failures dont match: %v != %v", f, c.MaxFailures)
	}

	signer := threshsign2.NewThreshsignEnclave(c.EnclavePath, uint32(3*f+int(c.MaxFastFailures)+1), uint32(len(c.Peers)))
	signer.Init(int(c.ID)+1, c.SecretKeys[c.ID], strings.Join(c.PublicKeys, ":"), c.ThreshsignFastLagrange)

	p := &sbft{
		id:    c.ID,
		nodes: c.Peers,

		log:     log,
		view:    pb.View(c.LeaderID),
		index:   0,
		execIdx: 0,

		active: false,
		logger: c.Logger,
		timers: make(map[peer.TickingTimer]struct{}),

		signer: signer,
		f:      f,
		c:      int(c.MaxFastFailures),
	}
	p.rQuorum = newRQuorum(p)
	p.initTimers()

	keys := []string{"tss_sign", "tss_agg", "tss_verify"}
	totalWorkers := 0
	for _, k := range keys {
		if _, ok := c.Workers[k]; !ok {
			p.logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
		}
		if count, ok := c.WorkersQueueSizes[k]; !ok {
			p.logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
			totalWorkers += int(count)
		}
	}
	p.callbackC = make(chan func(), totalWorkers)

	p.signDispatcher = worker.NewDispatcher(int(c.WorkersQueueSizes["tss_sign"]), int(c.Workers["tss_sign"]), p.callbackC)
	p.signDispatcher.Run()

	p.aggDispatcher = worker.NewDispatcher(int(c.WorkersQueueSizes["tss_agg"]), int(c.Workers["tss_agg"]), p.callbackC)
	p.aggDispatcher.Run()

	p.verifyDispatcher = worker.NewDispatcher(int(c.WorkersQueueSizes["tss_verify"]), int(c.Workers["tss_verify"]), p.callbackC)
	p.verifyDispatcher.Run()

	return p
}

func (s *sbft) initTimers() {
	// const sendTimeout = 5
	// asyncSignerTimer := peer.MakeTickingTimer(sendTimeout, func() {
	// 	select {
	// 	case s.asyncSigner.timerC <- true:
	// 		// s.logger.Debug("sending timer event")
	// 	default:
	// 		// s.logger.Debug("skipping timer event")
	// 	}
	// })
	// s.registerInfiniteTimer(asyncSignerTimer)
}

func (s *sbft) registerInfiniteTimer(t peer.TickingTimer) {
	s.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (s *sbft) Step(message peerpb.Message) {
	// recvCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, opentracing.TextMapReader(opentracing.TextMapCarrier(message.TraceInfo)))
	// if err != nil {
	// 	// s.logger.Panicf("Unable to extract trace: %w", err)
	// }
	// span := opentracing.StartSpan("Step", opentracing.ChildOf(recvCtx))
	// ctx := opentracing.ContextWithSpan(context.Background(), span)
	// defer span.Finish()

	sbftMsg := &pb.SBFTMessage{}
	if err := proto.Unmarshal(message.Content, sbftMsg); err != nil {
		panic(err)
	}

	switch t := sbftMsg.Type.(type) {
	case *pb.SBFTMessage_Normal:
		s.stepNormal(t.Normal, message.From, message.Certificate, message.Content)
	case *pb.SBFTMessage_Result:
		s.stepResult(t.Result, message.From)
	default:
		panic("Unknown message type")
	}
}

func (s *sbft) Callback(message peer.ExecCallback) {
	s.logger.Debugf("Received post-execute callback for command %v",
		message.Cmd.Timestamp)

	// recvCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap,
	// 	opentracing.TextMapCarrier(message.Cmd.TraceInfo))
	// if err != nil {
	// 	// s.logger.Panicf("Unable to extract trace from %v: %w", message.Cmd[0].TraceInfo, err)
	// }
	// span := opentracing.StartSpan("Callback", opentracing.ChildOf(recvCtx))
	// ctx := opentracing.ContextWithSpan(context.Background(), span)
	// defer span.Finish()

	msg := &pb.ResultMessage{
		Result: message.Result,
		Id:     message.Cmd.Timestamp,
	}

	rid := peerpb.PeerID(s.view) % peerpb.PeerID(len(s.nodes))
	s.logger.Debugf("Sending ResultMessage to %d for %d", rid,
		message.Cmd.Timestamp)
	if s.id != rid {
		mBytes := s.marshall(msg)
		s.sendTo(rid, mBytes, nil)
	} else {
		s.stepResult(msg, rid)
	}
}

func (s *sbft) stepResult(m *pb.ResultMessage, from peerpb.PeerID) {
	// span, ctx := opentracing.StartSpanFromContext(ctx, "stepResult")
	// defer span.Finish()

	s.logger.Debugf("Replica %d ====[ResultMessage,%d]====>>> Replica %d\n", from, m.Id, s.id)
	s.rQuorum.log(from, m)
	if s.rQuorum.Majority(m) {
		// span.SetTag("majority", true)
		s.executedCmds = append(s.executedCmds, peer.ExecPacket{Meta: m.Result, NoExec: true})
	}
}

func (s *sbft) stepNormal(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	s.logger.Debugf("Replica %d ====[%v,%v]====>>> Replica %d\n", from, m.CommandHash, m.Type, s.id)

	switch m.Type {
	case pb.NormalMessage_Prepare:
		s.onPrepare(m, from, sign)
	case pb.NormalMessage_SignShare:
		s.onSignShare(m, from, sign, content)
	case pb.NormalMessage_Commit:
		s.onCommit(m, from, sign, content)
	default:
		s.logger.Errorf("unknown message %v", m)
	}
}

func (s *sbft) Tick() {
	for t := range s.timers {
		t.Tick()
	}
}

func (s *sbft) Request(command *commandpb.Command) {
	recvCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap,
		opentracing.TextMapCarrier(command.TraceInfo))
	if err != nil {
		s.logger.Debugf("Unable to extract trace: %w", err)
	}
	command.TraceInfo = nil

	span := opentracing.StartSpan("Request", opentracing.ChildOf(recvCtx))
	ctx := opentracing.ContextWithSpan(context.Background(), span)
	defer span.Finish()

	s.onRequest(ctx, command)
}

func (s *sbft) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        s.msgs,
		OrderedCommands: s.executedCmds,
	}
}

func (s *sbft) ClearExecutedCommands() {
	s.executedCmds = nil
}

func (s *sbft) Execute(cmd *commandpb.Command) {
	s.executedCmds = append(s.executedCmds, peer.ExecPacket{Cmd: *cmd, Callback: true})
}

func (s *sbft) AsyncCallback() {
	for {
		select {
		case cb := <-s.callbackC:
			cb()
		default:
			return
		}
	}
}

func (s *sbft) hasPrepared(idx pb.Index) bool {
	return s.hasStatus(idx, pb.InstanceState_Prepared)
}

func (s *sbft) hasExecuted(idx pb.Index) bool {
	// TODO: Fix committed to executed
	return s.hasStatus(idx, pb.InstanceState_Executed)
}

func (s *sbft) hasStatus(idx pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := s.log[idx]; ok {
		return inst.is.Status == status
	}
	return false
}
