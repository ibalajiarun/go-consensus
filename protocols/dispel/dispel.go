package dispel

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
	"github.com/ibalajiarun/go-consensus/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	promReproposeCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "goconsensus",
		Subsystem: "server",
		Name:      "dispel_repropose_count",
		Help:      "Dispel Repropose Count",
	})
)

type Dispel struct {
	id          peerpb.PeerID
	peers       []peerpb.PeerID
	maxFailures int

	logger    logger.Logger
	certifier utils.Certifier

	nextEpochNumber  pb.Epoch
	epochs           map[pb.Epoch]*epoch
	nextDeliverEpoch pb.Epoch

	msgs      []peerpb.Message
	toDeliver []peer.ExecPacket

	timers map[peer.TickingTimer]struct{}

	rbThreshold int

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewDispel(cfg *peer.LocalConfig) *Dispel {
	d := &Dispel{
		id:          cfg.ID,
		peers:       cfg.Peers,
		maxFailures: int(cfg.MaxFailures),

		logger:    cfg.Logger,
		certifier: utils.NewSWCertifier(),

		epochs: make(map[pb.Epoch]*epoch),

		timers: make(map[peer.TickingTimer]struct{}),
	}
	if cfg.DispelWaitForAllRb {
		d.rbThreshold = len(cfg.Peers)
	} else {
		d.rbThreshold = len(cfg.Peers) - int(cfg.MaxFailures)
	}

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

func (d *Dispel) AsyncCallback() {
	for {
		select {
		case cb := <-d.callbackC:
			cb()
		default:
			return
		}
	}
}

func (d *Dispel) Callback(peer.ExecCallback) {
	panic("not implemented")
}

func (d *Dispel) ClearExecutedCommands() {
	d.toDeliver = nil
}

func (d *Dispel) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        d.msgs,
		OrderedCommands: d.toDeliver,
	}
}

func (d *Dispel) Request(cmd *commandpb.Command) {
	d.onRequest(cmd)
}

func (d *Dispel) Step(message peerpb.Message) {
	mirMsg := &pb.DispelMessage{}
	if err := proto.Unmarshal(message.Content, mirMsg); err != nil {
		panic(err)
	}

	if !d.certifier.VerifyCertificate(message.Content, 0, message.Certificate) {
		panic("Invalid certificate")
	}

	switch t := mirMsg.Type.(type) {
	case *pb.DispelMessage_Broadcast:
		d.stepBroadcast(t.Broadcast, message.From)
	case *pb.DispelMessage_Consensus:
		d.stepConsensus(t.Consensus, message.From)
	default:
		d.logger.Panicf("Unknown message: %v", mirMsg)
	}
}

func (d *Dispel) stepBroadcast(m *pb.RBMessage, from peerpb.PeerID) {
	d.logger.Debugf("Replica %d ====[%v, (E: %v, ID: %v)]====>>> Replica %d\n",
		from, m.Type, m.EpochNum, m.PeerID, d.id)

	switch m.Type {
	case pb.RBMessage_Send:
		d.onRBSend(m, from)
	case pb.RBMessage_Echo:
		d.onRBEcho(m, from)
	case pb.RBMessage_Ready:
		d.onRBReady(m, from)
	default:
		d.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (d *Dispel) stepConsensus(m *pb.ConsensusMessage, from peerpb.PeerID) {
	d.logger.Debugf("Replica %d ====[%v, (%v)]====>>> Replica %d\n", from, m.Type, m, d.id)

	switch m.Type {
	case pb.ConsensusMessage_Estimate:
		d.onBVBroadcast(m, from)
	case pb.ConsensusMessage_CoordValue:
		d.onCoordValue(m, from)
	case pb.ConsensusMessage_Aux:
		d.onAuxMessage(m, from)
	default:
		d.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (d *Dispel) Tick() {
	for t := range d.timers {
		t.Tick()
	}
}

func (d *Dispel) registerOneTimeTimer(t peer.TickingTimer) {
	d.timers[t] = struct{}{}
	t.Instrument(func() {
		d.unregisterTimer(t)
	})
	t.Reset()
}

func (d *Dispel) unregisterTimer(t peer.TickingTimer) {
	t.Stop()
	delete(d.timers, t)
}

func (d *Dispel) enqueueForExecution(cmd *commandpb.Command) {
	d.toDeliver = append(d.toDeliver, peer.ExecPacket{Cmd: *cmd})
}

func (d *Dispel) hasExecuted(cmd *commandpb.Command) bool {
	for _, pkt := range d.toDeliver {
		if pkt.Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}

func (d *Dispel) hasBroadcast(cmd *commandpb.Command) bool {
	for _, ep := range d.epochs {
		rb, ok := ep.rbs[cmd.Target]
		if ok && rb.rbs.Status == pb.RBState_Readied {
			return true
		}
	}
	return false
}

var _ peer.Protocol = (*Dispel)(nil)
