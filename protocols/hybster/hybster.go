package hybster

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/trinx"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybster/hybsterpb"
)

type hybster struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID
	f     int

	log     map[pb.Order]*instance // log ordered by slot
	execute pb.Order               // next execute slot number
	active  bool                   // active leader
	slot    pb.Order               // highest slot number

	msgs         []peerpb.Message
	executedCmds []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier *trinx.TrInxEnclave

	omsgs   []prPkg
	vmsgs   []vcPkg
	nvmsgs  []pb.NewViewMessage
	nvamsgs []nvaPkg

	stableView pb.View
	curView    pb.View

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewHybster(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Order]*instance, 1024)
	log[0] = &instance{}
	p := &hybster{
		id:    c.ID,
		nodes: c.Peers,
		// f:       len(c.Peers) / 2,
		f:       int(c.MaxFailures),
		log:     log,
		slot:    0,
		execute: 1,
		timers:  make(map[peer.TickingTimer]struct{}),
		logger:  c.Logger,

		certifier: trinx.NewTrInxEnclave(c.EnclavePath),

		omsgs:   make([]prPkg, 0, 1024),
		vmsgs:   make([]vcPkg, 0, 1024),
		nvmsgs:  make([]pb.NewViewMessage, 0, 10),
		nvamsgs: make([]nvaPkg, 0, 10),

		curView:    pb.View(c.LeaderID),
		stableView: pb.View(c.LeaderID),
	}
	p.certifier.Init([]byte(c.SecretKeys[0]), 3, len(p.nodes))

	p.callbackC = make(chan func(), c.WorkersQueueSizes["mac_sign"])
	p.signDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		1,
		p.callbackC,
	)
	p.signDispatcher.Run()

	return p
}

func (p *hybster) Step(mm peerpb.Message) {
	m := &pb.HybsterMessage{}
	if err := proto.Unmarshal(mm.Content, m); err != nil {
		panic(err)
	}

	p.logger.Debugf("Received from %v: %v", mm.From, m)

	switch t := m.Type.(type) {
	case *pb.HybsterMessage_Normal:
		p.stepNormal(t.Normal, mm.Content, mm.Certificate, mm.From)
	case *pb.HybsterMessage_ViewChange:
		p.onViewChange(t.ViewChange, mm.Content, mm.Certificate, mm.From)
	case *pb.HybsterMessage_NewView:
		p.onNewView(t.NewView, mm.Content, mm.Certificate, mm.From)
	case *pb.HybsterMessage_NewViewAck:
		p.onNewViewAck(t.NewViewAck, mm.Content, mm.Certificate, mm.From)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *hybster) stepNormal(m *pb.NormalMessage, content, cert []byte, from peerpb.PeerID) {
	if p.curView != p.stableView {
		p.logger.Infof("View change in progress. Ignoring %v", m)
		return
	}

	counter := uint64(uint64(m.View<<48) | uint64(m.Order))
	if !p.certifier.VerifyIndependentCounterCertificate(content, cert, counter, 0) {
		p.logger.Errorf("Invalid certificate")
		return
	}

	if m.View != p.curView {
		p.logger.Fatalf("inconsistent views %v != %v", m.View, p.curView)
	}

	switch m.Type {
	case pb.NormalMessage_Prepare:
		p.omsgs = append(p.omsgs, prPkg{m, content, cert, from})
		p.onPrepare(m, from)
	case pb.NormalMessage_Commit:
		p.onCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *hybster) Callback(message peer.ExecCallback) {
}

func (p *hybster) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *hybster) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *hybster) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.executedCmds,
	}
}

func (p *hybster) ClearExecutedCommands() {
	p.executedCmds = nil
}

func (p *hybster) Execute(cmd *commandpb.Command) {
	p.executedCmds = append(p.executedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (p *hybster) AsyncCallback() {
	for {
		select {
		case cb := <-p.callbackC:
			cb()
		default:
			return
		}
	}
}

func (p *hybster) Stop() {
	p.certifier.Destroy()
}

func (p *hybster) hasPrepared(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		return inst.prepared
	}
	return false
}

func (p *hybster) hasCommitted(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		return inst.prepared && inst.commit
	}
	return false
}

func (p *hybster) hasExecuted(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		return inst.prepared && inst.commit && inst.executed
	}
	return false
}

func (p *hybster) leaderOfView(v pb.View) peerpb.PeerID {
	return peerpb.PeerID(v) % peerpb.PeerID(len(p.nodes))
}
