package linhybster

import (
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/threshsign2"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/linhybster/linhybsterpb"
)

type linhybster struct {
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

	executedCmds []peer.ExecPacket
	rQuorum      *rquorum

	cReqBuffer []peerpb.PeerID
	oBatchSize uint32

	signDispatcher   *worker.Dispatcher
	aggDispatcher    *worker.Dispatcher
	verifyDispatcher *worker.Dispatcher
	callbackC        chan func()
}

func NewLinHybster(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Index]*instance, 1024)

	signer := threshsign2.NewThreshsignEnclave(c.EnclavePath, uint32(c.MaxFailures+1), uint32((2*c.MaxFailures)+1))
	signer.Init(int(c.ID)+1, c.SecretKeys[c.ID], strings.Join(c.PublicKeys, ":"), c.ThreshsignFastLagrange)

	p := &linhybster{
		id:    c.ID,
		nodes: c.Peers,

		log:     log,
		view:    pb.View(c.LeaderID),
		index:   0,
		execIdx: 0,

		active: false,
		logger: c.Logger,
		timers: make(map[peer.TickingTimer]struct{}),

		// f: (len(c.Peers) - 1) / 2,
		f: int(c.MaxFailures),

		signer: signer,

		oBatchSize: c.DqOBatchSize,
	}
	p.rQuorum = newRQuorum(p)
	p.initTimers()

	keys := []string{"tss_sign", "tss_agg", "tss_verify"}
	totalWorkers := 0
	for _, k := range keys {
		if _, ok := c.Workers[k]; !ok {
			p.logger.Panicf("Worker count for key missing: %s. Required: %v",
				k, keys)
		}
		if count, ok := c.WorkersQueueSizes[k]; !ok {
			p.logger.Panicf("Worker count for key missing: %s. Required: %v",
				k, keys)
			totalWorkers += int(count)
		}
	}
	p.callbackC = make(chan func(), totalWorkers)

	p.signDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["tss_sign"]),
		1, p.callbackC)
	p.signDispatcher.Run()

	p.aggDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["tss_agg"]),
		int(c.Workers["tss_agg"]), p.callbackC)
	p.aggDispatcher.Run()

	p.verifyDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["tss_verify"]),
		int(c.Workers["tss_verify"]), p.callbackC)
	p.verifyDispatcher.Run()

	return p
}

func (s *linhybster) initTimers() {
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

func (s *linhybster) registerInfiniteTimer(t peer.TickingTimer) {
	s.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (s *linhybster) Step(message peerpb.Message) {
	sdbftMsg := &pb.LinHybsterMessage{}
	if err := proto.Unmarshal(message.Content, sdbftMsg); err != nil {
		panic(err)
	}

	switch t := sdbftMsg.Type.(type) {
	case *pb.LinHybsterMessage_Normal:
		s.stepNormal(t.Normal, message.From, message.Certificate, message.Content)
	case *pb.LinHybsterMessage_Result:
		s.stepResult(t.Result, message.From)
	default:
		panic("Unknown message type")
	}
}

func (s *linhybster) Callback(message peer.ExecCallback) {
	s.logger.Debugf("Received post-execute callback for command %v",
		message.Cmd.Timestamp)
	rid := message.Cmd.Target
	msg := &pb.ResultMessage{
		Result: message.Result,
		Id:     message.Cmd.Timestamp,
	}
	s.logger.Debugf("Sending ResultMessage to %d for %d", rid,
		message.Cmd.Timestamp)
	if s.id != rid {
		mBytes := s.marshall(msg)
		s.sendTo(rid, mBytes, nil)
	} else {
		s.stepResult(msg, rid)
	}
}

func (s *linhybster) stepResult(m *pb.ResultMessage, from peerpb.PeerID) {
	s.logger.Debugf("Replica %d ====[ResultMessage,%d]====>>> Replica %d\n", from, m.Id, s.id)
	s.rQuorum.log(from, m)
	if s.rQuorum.Majority(m) && !s.rQuorum.GetAndSetExec(m) {
		s.executedCmds = append(s.executedCmds, peer.ExecPacket{Meta: m.Result, NoExec: true})
	}
}

func (s *linhybster) stepNormal(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
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

func (s *linhybster) Tick() {
	for t := range s.timers {
		t.Tick()
	}
}

func (s *linhybster) Request(command *commandpb.Command) {
	s.onRequest(command)
}

func (s *linhybster) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        s.msgs,
		OrderedCommands: s.executedCmds,
	}
}

func (s *linhybster) ClearExecutedCommands() {
	s.executedCmds = nil
}

func (s *linhybster) Execute(cmd *commandpb.Command) {
	s.executedCmds = append(s.executedCmds, peer.ExecPacket{Cmd: *cmd, Callback: true})
}

func (s *linhybster) AsyncCallback() {
	for {
		select {
		case cb := <-s.callbackC:
			cb()
		default:
			return
		}
	}
}

func (s *linhybster) hasPrepared(insId pb.Index) bool {
	return s.hasStatus(insId, pb.InstanceState_Prepared)
}

func (s *linhybster) hasExecuted(insId pb.Index) bool {
	return s.hasStatus(insId, pb.InstanceState_Executed)
}

func (s *linhybster) hasStatus(insId pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := s.log[insId]; ok {
		return inst.is.Status == status
	}
	return false
}
