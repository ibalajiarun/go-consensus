package dqpbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
	"github.com/ibalajiarun/go-consensus/utils"
)

type DQPBFT struct {
	id    peerpb.PeerID
	peers []peerpb.PeerID
	f     int

	log              map[pb.InstanceID]*instance
	view             pb.View
	index            pb.Index
	nextDeliverIndex map[peerpb.PeerID]pb.Index

	olog              map[pb.Index]*oinstance
	oindex            pb.Index
	oview             pb.View
	nextODeliverIndex pb.Index

	msgs      []peerpb.Message
	toDeliver []peer.ExecPacket

	cReqBuffer    []pb.InstanceID
	oBatchSize    uint32
	oBatchTimeout uint32

	timers map[peer.TickingTimer]struct{}
	logger logger.Logger

	certifier utils.Certifier

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewDQPBFT(cfg *peer.LocalConfig) *DQPBFT {
	log := make(map[pb.InstanceID]*instance, len(cfg.Peers)*1024)
	olog := make(map[pb.Index]*oinstance, len(cfg.Peers)*1024)

	d := &DQPBFT{
		id:    cfg.ID,
		peers: cfg.Peers,

		log:              log,
		view:             pb.View(cfg.ID),
		index:            0,
		nextDeliverIndex: make(map[peerpb.PeerID]pb.Index, len(cfg.Peers)),

		olog:              olog,
		oview:             pb.View(cfg.LeaderID),
		oindex:            0,
		nextODeliverIndex: 0,

		logger: cfg.Logger,
		timers: make(map[peer.TickingTimer]struct{}),

		f: int(cfg.MaxFailures),

		certifier: utils.NewSWCertifier(),

		oBatchSize:    cfg.DqOBatchSize,
		oBatchTimeout: cfg.DqOBatchTimeout,
	}

	d.initTimers()

	if _, ok := cfg.Workers["mac_sign"]; !ok {
		d.logger.Panicf("Worker count for mac_sign missing.")
	}
	if _, ok := cfg.WorkersQueueSizes["mac_sign"]; !ok {
		d.logger.Panicf("Worker queue size for mac_sign missing.")
	}

	d.callbackC = make(chan func(), cfg.WorkersQueueSizes["mac_sign"])
	d.signDispatcher = worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mac_sign"]),
		int(cfg.Workers["mac_sign"]),
		d.callbackC,
	)
	d.signDispatcher.Run()

	return d
}

func (d *DQPBFT) initTimers() {
	cReqTimer := peer.MakeTickingTimer(int(d.oBatchTimeout), func() {
		if d.isPrimaryAtView(d.id, d.oview) {
			d.sendCRequest()
		}
	})
	d.registerInfiniteTimer(cReqTimer)
}

func (s *DQPBFT) registerInfiniteTimer(t peer.TickingTimer) {
	s.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (d *DQPBFT) AsyncCallback() {
	for {
		select {
		case cb := <-d.callbackC:
			cb()
		default:
			return
		}
	}
}

func (d *DQPBFT) Callback(peer.ExecCallback) {

}

func (d *DQPBFT) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        d.msgs,
		OrderedCommands: d.toDeliver,
	}
}

func (d *DQPBFT) Request(cmd *commandpb.Command) {
	d.onRequest(cmd)
}

func (d *DQPBFT) Step(mm peerpb.Message) {
	m := &pb.DQPBFTMessage{}
	if err := proto.Unmarshal(mm.Content, m); err != nil {
		panic(err)
	}

	if !d.certifier.VerifyCertificate(mm.Content, 0, mm.Certificate) {
		panic("Invalid certificate")
	}

	d.logger.Debugf("Received from %v: %v", mm.From, m)

	switch t := m.Type.(type) {
	case *pb.DQPBFTMessage_Agreement:
		d.stepNormal(t.Agreement, mm.From)
	case *pb.DQPBFTMessage_OAgreement:
		d.stepONormal(t.OAgreement, mm.From)
	default:
		d.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (d *DQPBFT) stepNormal(m *pb.AgreementMessage, from peerpb.PeerID) {
	d.logger.Debugf("Replica %d ====[%v]====>>> Replica %d\n", from, m.Type, d.id)

	switch m.Type {
	case pb.AgreementMessage_PrePrepare:
		d.onPrePrepare(m, from)
	case pb.AgreementMessage_Prepare:
		d.onPrepare(m, from)
	case pb.AgreementMessage_Commit:
		d.onCommit(m, from)
	default:
		d.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (d *DQPBFT) stepONormal(m *pb.OAgreementMessage, from peerpb.PeerID) {
	d.logger.Debugf("Replica %d ====[%v]====>>> Replica %d\n", from, m.Type, d.id)

	switch m.Type {
	case pb.OAgreementMessage_OPrePrepare:
		d.onOPrePrepare(m, from)
	case pb.OAgreementMessage_OPrepare:
		d.onOPrepare(m, from)
	case pb.OAgreementMessage_OCommit:
		d.onOCommit(m, from)
	default:
		d.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (d *DQPBFT) Tick() {
	for t := range d.timers {
		t.Tick()
	}
}

var _ peer.Protocol = (*DQPBFT)(nil)

func (s *DQPBFT) hasExecuted(insId pb.InstanceID) bool {
	return s.hasStatus(insId, pb.InstanceState_Executed)
}

func (s *DQPBFT) hasStatus(insId pb.InstanceID, status pb.InstanceState_Status) bool {
	if inst, ok := s.log[insId]; ok {
		return inst.is.Status == status
	}
	return false
}
