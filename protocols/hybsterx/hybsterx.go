package hybsterx

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/trinx"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/bucketworker"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

type hybsterx struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID
	f     int

	log     map[pb.InstanceID]*instance
	view    pb.View
	index   pb.Index
	execIdx map[peerpb.PeerID]pb.Index

	olog     map[pb.Index]*oinstance
	oindex   pb.Index
	oview    pb.View
	oexecIdx pb.Index

	msgs              []peerpb.Message
	committedCommands []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier *trinx.TrInxEnclave

	cReqBuffer []pb.InstanceID
	oBatchSize uint32

	signDispatcher  *bucketworker.BucketDispatcher
	callbackC       chan func()
	signWorkerCount int
}

func NewHybsterx(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.InstanceID]*instance, len(c.Peers))
	olog := make(map[pb.Index]*oinstance, len(c.Peers))

	p := &hybsterx{
		id:    c.ID,
		nodes: c.Peers,

		f: int(c.MaxFailures),

		log:     log,
		view:    pb.View(c.ID),
		index:   0,
		execIdx: make(map[peerpb.PeerID]pb.Index, len(c.Peers)),

		olog:     olog,
		oview:    pb.View(c.LeaderID),
		oindex:   0,
		oexecIdx: 0,

		timers: make(map[peer.TickingTimer]struct{}),
		logger: c.Logger,

		certifier: trinx.NewTrInxEnclave(c.EnclavePath),

		oBatchSize: c.DqOBatchSize,
	}
	p.certifier.Init([]byte(c.SecretKeys[0]), len(p.nodes)+1, len(p.nodes))
	p.initTimers()

	p.callbackC = make(chan func(), c.WorkersQueueSizes["mac_sign"])
	p.signWorkerCount = int(c.Workers["mac_sign"])
	p.signDispatcher = bucketworker.NewBucketDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		int(c.Workers["mac_sign"]),
		p.callbackC,
	)
	p.signDispatcher.Run()

	return p
}

func (p *hybsterx) initTimers() {
	const sendTimeout = 5
	cReqTimer := peer.MakeTickingTimer(sendTimeout, func() {
		if p.isPrimaryAtView(p.id, p.oview) {
			p.sendCRequest()
		}
	})
	p.registerInfiniteTimer(cReqTimer)

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

func (p *hybsterx) registerInfiniteTimer(t peer.TickingTimer) {
	p.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (p *hybsterx) Step(mm peerpb.Message) {
	m := &pb.HybsterxMessage{}
	if err := proto.Unmarshal(mm.Content, m); err != nil {
		panic(err)
	}

	p.logger.Debugf("Received from %v: %v", mm.From, m)

	switch t := m.Type.(type) {
	case *pb.HybsterxMessage_Normal:
		p.stepNormal(t.Normal, mm.Content, mm.Certificate, mm.From)
	case *pb.HybsterxMessage_ONormal:
		p.stepONormal(t.ONormal, mm.Content, mm.Certificate, mm.From)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *hybsterx) stepNormal(m *pb.NormalMessage, content, cert []byte, from peerpb.PeerID) {
	counter := uint64(uint64(m.View<<48)|uint64(m.InstanceID.Index)) + 1
	if !p.certifier.VerifyIndependentCounterCertificate(content, cert, counter, int(m.InstanceID.PeerID)) {
		p.logger.Errorf("Invalid certificate")
		return
	}

	switch m.Type {
	case pb.NormalMessage_Prepare:
		p.onPrepare(m, from)
	case pb.NormalMessage_Commit:
		p.onCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *hybsterx) stepONormal(m *pb.ONormalMessage, content, cert []byte, from peerpb.PeerID) {
	counter := uint64(uint64(m.View<<48)|uint64(m.Index)) + 1
	if !p.certifier.VerifyIndependentCounterCertificate(content, cert, counter, len(p.nodes)) {
		p.logger.Errorf("Invalid certificate")
		return
	}

	switch m.Type {
	case pb.ONormalMessage_OPrepare:
		p.onOPrepare(m, from)
	case pb.ONormalMessage_OCommit:
		p.onOCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *hybsterx) Callback(message peer.ExecCallback) {
}

func (p *hybsterx) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *hybsterx) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *hybsterx) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.committedCommands,
	}
}

func (p *hybsterx) ClearExecutedCommands() {
	p.committedCommands = nil
}

func (p *hybsterx) Execute(cmd *commandpb.Command) {
	p.committedCommands = append(p.committedCommands, peer.ExecPacket{Cmd: *cmd})
}

func (p *hybsterx) AsyncCallback() {
	for {
		select {
		case cb := <-p.callbackC:
			cb()
		default:
			return
		}
	}
}

func (p *hybsterx) Stop() {
	p.certifier.Destroy()
}

func (p *hybsterx) hasPrepared(insId pb.InstanceID) bool {
	return p.hasStatus(insId, pb.InstanceState_Prepared)
}

func (p *hybsterx) hasExecuted(insId pb.InstanceID) bool {
	return p.hasStatus(insId, pb.InstanceState_Executed)
}

func (p *hybsterx) hasStatus(insId pb.InstanceID, status pb.InstanceState_Status) bool {
	if inst, ok := p.log[insId]; ok {
		return inst.is.Status == status
	}
	return false
}

func (p *hybsterx) leaderOfView(v pb.View) peerpb.PeerID {
	return peerpb.PeerID(v) % peerpb.PeerID(len(p.nodes))
}
