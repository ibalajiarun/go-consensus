package dqsbftslow

import (
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/threshsign2"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqsbftslow/dqsbftslowpb"
)

type dqsbftslow struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	log     map[pb.InstanceID]*instance
	view    pb.View
	index   pb.Index
	execIdx map[peerpb.PeerID]pb.Index

	olog     map[pb.Index]*oinstance
	oindex   pb.Index
	oview    pb.View
	oexecIdx pb.Index

	active bool

	msgs              []peerpb.Message
	committedCommands []peer.ExecPacket

	logger logger.Logger

	timers           map[peer.TickingTimer]struct{}
	pendingInstances []pb.InstanceID

	signer *threshsign2.ThreshsignEnclave

	f  int
	ff int

	executedCmds []peer.ExecPacket
	rQuorum      *rquorum

	cReqBuffer    []pb.InstanceID
	oBatchSize    uint32
	oBatchTimeout uint32

	signDispatcher   *worker.Dispatcher
	aggDispatcher    *worker.Dispatcher
	verifyDispatcher *worker.Dispatcher
	callbackC        chan func()
}

func NewDQSBFTSlow(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.InstanceID]*instance, len(c.Peers))
	olog := make(map[pb.Index]*oinstance, len(c.Peers))

	f := int(c.MaxFailures)
	signer := threshsign2.NewThreshsignEnclave(c.EnclavePath, uint32(2*f+int(c.MaxFastFailures)+1), uint32(3*f+int(c.MaxFastFailures)+1))
	signer.Init(int(c.ID)+1, c.SecretKeys[c.ID], strings.Join(c.PublicKeys, ":"), c.ThreshsignFastLagrange)

	p := &dqsbftslow{
		id:    c.ID,
		nodes: c.Peers,

		log:     log,
		view:    pb.View(c.ID),
		index:   0,
		execIdx: make(map[peerpb.PeerID]pb.Index, len(c.Peers)),

		olog:     olog,
		oview:    pb.View(c.LeaderID),
		oindex:   0,
		oexecIdx: 0,

		active: false,
		logger: c.Logger,
		timers: make(map[peer.TickingTimer]struct{}),

		// f: (len(c.Peers) - 1) / 2,
		f:  int(c.MaxFailures),
		ff: int(c.MaxFastFailures),

		signer: signer,

		oBatchSize:    c.DqOBatchSize,
		oBatchTimeout: c.DqOBatchTimeout,
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

	p.signDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["tss_sign"]),
		int(c.Workers["tss_sign"]), p.callbackC)
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

func (s *dqsbftslow) initTimers() {
	cReqTimer := peer.MakeTickingTimer(int(s.oBatchTimeout), func() {
		if s.isPrimaryAtView(s.id, s.oview) {
			s.sendCRequest()
		}
	})
	s.registerInfiniteTimer(cReqTimer)

	// const checkTimeout = 5000
	// checkTimer := peer.MakeTickingTimer(checkTimeout, func() {
	// 	// s.logger.Errorf("Check Timer...")
	// 	for _, instID := range s.pendingInstances {
	// 		inst := s.log[instID]
	// 		if inst.is.Status < pb.InstanceState_Committed {
	// 			s.logger.Errorf("Instance %v not commited yet: %v; votes: %v", instID, inst, inst.cCert.msgs[0])
	// 		}
	// 	}
	// 	s.pendingInstances = nil
	// 	for instID, inst := range s.log {
	// 		if inst.is.Status < pb.InstanceState_Committed {
	// 			s.pendingInstances = append(s.pendingInstances, instID)
	// 		}
	// 	}
	// 	// s.logger.Errorf("=============================")
	// })
	// s.registerInfiniteTimer(checkTimer)
}

func (s *dqsbftslow) registerInfiniteTimer(t peer.TickingTimer) {
	s.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (s *dqsbftslow) Step(message peerpb.Message) {
	sdbftMsg := &pb.DQSBFTSlowMessage{}
	if err := proto.Unmarshal(message.Content, sdbftMsg); err != nil {
		panic(err)
	}

	switch t := sdbftMsg.Type.(type) {
	case *pb.DQSBFTSlowMessage_Normal:
		s.stepNormal(t.Normal, message.From, message.Certificate, message.Content)
	case *pb.DQSBFTSlowMessage_Result:
		s.stepResult(t.Result, message.From)
	case *pb.DQSBFTSlowMessage_ONormal:
		s.stepONormal(t.ONormal, message.From, message.Certificate, message.Content)
	default:
		panic("Unknown message type")
	}
}

func (s *dqsbftslow) Callback(message peer.ExecCallback) {
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

func (s *dqsbftslow) stepResult(m *pb.ResultMessage, from peerpb.PeerID) {
	s.logger.Debugf("Replica %d ====[ResultMessage,%d]====>>> Replica %d\n", from, m.Id, s.id)
	s.rQuorum.log(from, m)
	if s.rQuorum.Majority(m) && !s.rQuorum.GetAndSetExec(m) {
		s.executedCmds = append(s.executedCmds, peer.ExecPacket{Meta: m.Result, NoExec: true})
	}
}

func (s *dqsbftslow) stepNormal(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	s.logger.Debugf("Replica %d ====[%v,%v]====>>> Replica %d\n", from, m.CommandHash, m.Type, s.id)

	switch m.Type {
	case pb.NormalMessage_Preprepare:
		s.onPreprepare(m, from, sign)
	case pb.NormalMessage_SignShare:
		s.onSignShare(m, from, sign, content)
	case pb.NormalMessage_Prepare:
		s.onPrepare(m, from, sign)
	case pb.NormalMessage_CommitSig:
		s.onCommitSig(m, from, sign, content)
	case pb.NormalMessage_CommitSlow:
		s.onCommit(m, from, sign, content)
	default:
		s.logger.Errorf("unknown message %v", m)
	}
}

func (s *dqsbftslow) stepONormal(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	s.logger.Debugf("Replica %d ====[%v,%v]====>>> Replica %d\n", from, m.Instances, m.Type, s.id)

	switch m.Type {
	case pb.ONormalMessage_Preprepare:
		s.onOPreprepare(m, from, sign)
	case pb.ONormalMessage_SignShare:
		s.onOSignShare(m, from, sign, content)
	case pb.ONormalMessage_Prepare:
		s.onOPrepare(m, from, sign)
	case pb.ONormalMessage_CommitSig:
		s.onOCommitSig(m, from, sign, content)
	case pb.ONormalMessage_CommitSlow:
		s.onOCommit(m, from, sign, content)
	default:
		s.logger.Errorf("unknown message %v", m)
	}
}

func (s *dqsbftslow) Tick() {
	for t := range s.timers {
		t.Tick()
	}
}

func (s *dqsbftslow) Request(command *commandpb.Command) {
	s.onRequest(command)
}

func (s *dqsbftslow) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        s.msgs,
		OrderedCommands: s.executedCmds,
	}
}

func (s *dqsbftslow) ClearExecutedCommands() {
	s.executedCmds = nil
}

func (s *dqsbftslow) Execute(cmd *commandpb.Command) {
	s.executedCmds = append(s.executedCmds, peer.ExecPacket{Cmd: *cmd, Callback: true})
}

func (s *dqsbftslow) AsyncCallback() {
	for {
		select {
		case cb := <-s.callbackC:
			cb()
		default:
			return
		}
	}
}

func (s *dqsbftslow) hasPrepared(insId pb.InstanceID) bool {
	return s.hasStatus(insId, pb.InstanceState_Prepared)
}

func (s *dqsbftslow) hasExecuted(insId pb.InstanceID) bool {
	return s.hasStatus(insId, pb.InstanceState_Executed)
}

func (s *dqsbftslow) hasStatus(insId pb.InstanceID, status pb.InstanceState_Status) bool {
	if inst, ok := s.log[insId]; ok {
		return inst.is.Status == status
	}
	return false
}
