package prime

import (
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
	"github.com/ibalajiarun/go-consensus/utils"
)

var (
	poSummaryTimeout = 5
)

type prime struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	log                       map[pb.InstanceID]*instance // log ordered by index
	poSummary                 []pb.Index
	lastPreorderSummaries     pb.POSummaryMatrix
	lastPreorderSummariesHash []byte
	lastPrepareHash           []byte
	prePrepareCounter         int

	index   pb.Index // highest index number
	execIdx map[peerpb.PeerID]pb.Index

	active bool    // active leader
	oview  pb.View // highest view number
	oindex pb.Index
	olog   map[pb.Index]*oinstance

	oexecIdx pb.Index // next execute index number

	msgs         []peerpb.Message
	executedCmds []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier utils.Certifier

	f int

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewPrime(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.InstanceID]*instance, len(c.Peers))

	lastPreorderSummaries := pb.POSummaryMatrix{POSummaryMatrix: make([]pb.POSummary, len(c.Peers))}

	p := &prime{
		id:                    c.ID,
		nodes:                 c.Peers,
		log:                   log,
		poSummary:             make([]pb.Index, len(c.Peers)),
		lastPreorderSummaries: lastPreorderSummaries,
		execIdx:               make(map[peerpb.PeerID]pb.Index, len(c.Peers)),
		index:                 1,

		olog:     make(map[pb.Index]*oinstance),
		oexecIdx: 1,
		oview:    pb.View(c.LeaderID),
		oindex:   1,

		timers: make(map[peer.TickingTimer]struct{}),
		logger: c.Logger,

		certifier: utils.NewSWCertifier(),
		// certifier: utils.NewDummyCertifier(),

		f: int(c.MaxFailures),
	}
	for i := range c.Peers {
		p.lastPreorderSummaries.POSummaryMatrix[i] = pb.POSummary{
			POSummary: make([]pb.Index, len(c.Peers)),
		}
		p.execIdx[peerpb.PeerID(i)] = 1
	}
	mat := &pb.POSummaryMatrix{POSummaryMatrix: make([]pb.POSummary, len(p.lastPreorderSummaries.POSummaryMatrix))}
	copy(mat.POSummaryMatrix, p.lastPreorderSummaries.POSummaryMatrix)

	matBytes, err := proto.Marshal(mat)
	if err != nil {
		panic(err)
	}
	matHash := sha256.Sum256(matBytes)

	p.lastPreorderSummariesHash = matHash[:]
	p.lastPrepareHash = matHash[:]

	p.certifier.Init()
	p.initTimers()

	if _, ok := c.Workers["mac_sign"]; !ok {
		p.logger.Panicf("Worker count for mac_sign missing.")
	}
	if _, ok := c.WorkersQueueSizes["mac_sign"]; !ok {
		p.logger.Panicf("Worker queue size for mac_sign missing.")
	}

	p.callbackC = make(chan func(), c.WorkersQueueSizes["mac_sign"])
	p.signDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		int(c.Workers["mac_sign"]),
		p.callbackC,
	)
	p.signDispatcher.Run()

	return p
}

func (p *prime) initTimers() {
	// const startProcess = 100
	// startProcessTimer := peer.MakeTickingTimer(startProcess, func() {
	poSummaryTimer := peer.MakeTickingTimer(poSummaryTimeout, func() {
		p.sendSummary()
	})
	p.registerInfiniteTimer(poSummaryTimer)

	const preprepareTimeout = 5
	preprepareTimer := peer.MakeTickingTimer(preprepareTimeout, func() {
		if p.isPrimaryAtView(p.id, p.oview) {
			p.sendPrePrepare()
		}
	})
	p.registerInfiniteTimer(preprepareTimer)
	// })
	// p.registerOneTimeTimer(startProcessTimer)
}

func (p *prime) registerInfiniteTimer(t peer.TickingTimer) {
	p.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (p *prime) registerOneTimeTimer(t peer.TickingTimer) {
	p.timers[t] = struct{}{}
	t.Instrument(func() {
		p.unregisterTimer(t)
	})
	t.Reset()
}

func (p *prime) unregisterTimer(t peer.TickingTimer) {
	t.Stop()
	delete(p.timers, t)
}

func (p *prime) Step(message peerpb.Message) {
	primeMsg := &pb.PrimeMessage{}
	if err := proto.Unmarshal(message.Content, primeMsg); err != nil {
		panic(err)
	}

	if !p.certifier.VerifyCertificate(message.Content, 0, message.Certificate) {
		panic("Invalid certificate")
	}

	switch t := primeMsg.Type.(type) {
	case *pb.PrimeMessage_Preorder:
		p.stepPreOrder(t.Preorder, message.From)
	case *pb.PrimeMessage_Order:
		p.stepOrder(t.Order, message.From)
	}
}

func (p *prime) Callback(message peer.ExecCallback) {
	panic("not implemented")
}

func (p *prime) stepPreOrder(m *pb.PreOrderMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %d ====[%v]====>>> Replica %d\n", from, m.Type, p.id)

	switch m.Type {
	case pb.PreOrderMessage_PORequest:
		p.onPORequest(m, from)
	case pb.PreOrderMessage_POAck:
		p.onPOAck(m, from)
	case pb.PreOrderMessage_POSummary:
		p.onPOSummary(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *prime) stepOrder(m *pb.OrderMessage, from peerpb.PeerID) {
	// Check if this node is in the same view as the message
	if m.View != p.oview {
		p.logger.Errorf("Views do not match. Has: %v, Received: %v", p.oview, m.View)
		return
	}

	p.logger.Debugf("Replica %d ====[%v,%v,%v]====>>> Replica %d\n", from, m.Type, m.Index, m.POSummaryMatrix, p.id)

	switch m.Type {
	case pb.OrderMessage_PrePrepare:
		p.onPrePrepare(m, from)
	case pb.OrderMessage_Prepare:
		p.onPrepare(m, from)
	case pb.OrderMessage_Commit:
		p.onCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *prime) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *prime) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *prime) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.executedCmds,
	}
}

func (p *prime) ClearExecutedCommands() {
	p.executedCmds = nil
}

func (p *prime) Execute(cmd *commandpb.Command) {
	p.executedCmds = append(p.executedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (p *prime) AsyncCallback() {
	for {
		select {
		case cb := <-p.callbackC:
			cb()
		default:
			return
		}
	}
}

func (p *prime) Stop() {
	p.certifier.Destroy()
}

func (p *prime) hasPreOrdered(instID pb.InstanceID) bool {
	return p.hasStatus(instID, pb.InstanceState_PreOrder)
}

func (p *prime) hasPOAcked(instID pb.InstanceID) bool {
	// TODO: Fix committed to executed
	return p.hasStatus(instID, pb.InstanceState_PreOrderAck)
}

func (p *prime) hasExecuted(instID pb.InstanceID) bool {
	return p.hasStatus(instID, pb.InstanceState_Executed)
}

func (p *prime) hasStatus(instID pb.InstanceID, status pb.InstanceState_Status) bool {
	if inst, ok := p.log[instID]; ok {
		return inst.is.Status == status
	}
	return false
}
