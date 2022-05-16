package sbftslow

import (
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/threshsign2"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbftslow/sbftslowpb"
)

type sbftslow struct {
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

func NewSBFTSlow(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Index]*instance, 1024)
	// f := (len(c.Peers) - 2*int(c.MaxFastFailures) - 1) / 3
	// if f < 1 {
	// 	c.Logger.Panicf("Invalid failure count: %v", f)
	// }
	// if f != int(c.MaxFailures) {
	// 	c.Logger.Panicf("Failures dont match: %v != %v", f, c.MaxFailures)
	// }
	f := int(c.MaxFailures)
	signer := threshsign2.NewThreshsignEnclave(c.EnclavePath, uint32(2*f+int(c.MaxFastFailures)+1), uint32(3*f+int(c.MaxFastFailures)+1))
	signer.Init(int(c.ID)+1, c.SecretKeys[c.ID], strings.Join(c.PublicKeys, ":"), c.ThreshsignFastLagrange)

	p := &sbftslow{
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

func (s *sbftslow) initTimers() {

}

func (s *sbftslow) registerInfiniteTimer(t peer.TickingTimer) {
	s.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (s *sbftslow) Step(message peerpb.Message) {
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

func (s *sbftslow) Callback(message peer.ExecCallback) {
	s.logger.Debugf("Received post-execute callback for command %v",
		message.Cmd.Timestamp)
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

func (s *sbftslow) stepResult(m *pb.ResultMessage, from peerpb.PeerID) {
	s.logger.Debugf("Replica %d ====[ResultMessage,%d]====>>> Replica %d\n", from, m.Id, s.id)
	s.rQuorum.log(from, m)
	if s.rQuorum.Majority(m) {
		s.executedCmds = append(s.executedCmds, peer.ExecPacket{Meta: m.Result, NoExec: true})
	}
}

func (s *sbftslow) stepNormal(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	s.logger.Debugf("Replica %d ====[%v,%v]====>>> Replica %d\n", from, m.Index, m.Type, s.id)

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

func (s *sbftslow) Tick() {
	for t := range s.timers {
		t.Tick()
	}
}

func (s *sbftslow) Request(command *commandpb.Command) {
	s.onRequest(command)
}

func (s *sbftslow) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        s.msgs,
		OrderedCommands: s.executedCmds,
	}
}

func (s *sbftslow) ClearExecutedCommands() {
	s.executedCmds = nil
}

func (s *sbftslow) Execute(cmd *commandpb.Command) {
	s.executedCmds = append(s.executedCmds, peer.ExecPacket{Cmd: *cmd, Callback: true})
}

func (s *sbftslow) AsyncCallback() {
	for {
		select {
		case cb := <-s.callbackC:
			cb()
		default:
			return
		}
	}
}

func (s *sbftslow) hasPrepared(idx pb.Index) bool {
	return s.hasStatus(idx, pb.InstanceState_Prepared)
}

func (s *sbftslow) hasExecuted(idx pb.Index) bool {
	// TODO: Fix committed to executed
	return s.hasStatus(idx, pb.InstanceState_Executed)
}

func (s *sbftslow) hasStatus(idx pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := s.log[idx]; ok {
		return inst.is.Status == status
	}
	return false
}
