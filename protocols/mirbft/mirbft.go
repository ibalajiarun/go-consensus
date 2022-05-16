package mirbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/mirbft/mirbftpb"
	"github.com/ibalajiarun/go-consensus/utils"
)

type MirBFT struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	log              map[pb.Index]*instance // log ordered by index
	nextDeliverIndex pb.Index               // next execute index number
	nextLocalIndex   pb.Index               // highest index number

	epoch        pb.Epoch // highest epoch number
	epochLeaders map[pb.Epoch]map[peerpb.PeerID]struct{}

	msgs      []peerpb.Message
	toDeliver []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier utils.Certifier

	viewChanging bool
	// changingView pb.Epoch
	// vcQuorum     *VCQuorum
	f int

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewMirBFT(cfg *peer.LocalConfig) *MirBFT {
	log := make(map[pb.Index]*instance, 200_000)
	mir := &MirBFT{
		id:     cfg.ID,
		nodes:  cfg.Peers,
		log:    log,
		timers: make(map[peer.TickingTimer]struct{}),
		logger: cfg.Logger,

		certifier: utils.NewSWCertifier(),

		// epoch:        pb.Epoch(cfg.LeaderID),
		epochLeaders: make(map[pb.Epoch]map[peerpb.PeerID]struct{}, 10),

		f: int(cfg.MaxFailures),
	}

	firstEpoch := make(map[peerpb.PeerID]struct{}, len(cfg.Peers))
	for _, id := range cfg.Peers {
		firstEpoch[id] = struct{}{}
	}
	mir.epochLeaders[pb.Epoch(0)] = firstEpoch
	mir.nextLocalIndex = pb.Index(mir.id)

	if _, ok := cfg.Workers["mac_sign"]; !ok {
		mir.logger.Panicf("Worker count for mac_sign missing.")
	}
	if _, ok := cfg.WorkersQueueSizes["mac_sign"]; !ok {
		mir.logger.Panicf("Worker queue size for mac_sign missing.")
	}

	mir.callbackC = make(chan func(), cfg.WorkersQueueSizes["mac_sign"])
	mir.signDispatcher = worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mac_sign"]),
		int(cfg.Workers["mac_sign"]),
		mir.callbackC,
	)
	mir.signDispatcher.Run()

	return mir
}

func (mir *MirBFT) AsyncCallback() {
	for {
		select {
		case cb := <-mir.callbackC:
			cb()
		default:
			return
		}
	}
}

func (mir *MirBFT) Callback(peer.ExecCallback) {
	panic("not implemented")
}

func (mir *MirBFT) ClearExecutedCommands() {
	mir.toDeliver = nil
}

func (mir *MirBFT) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        mir.msgs,
		OrderedCommands: mir.toDeliver,
	}
}

func (mir *MirBFT) Request(command *commandpb.Command) {
	mir.onRequest(command)
}

func (mir *MirBFT) Step(message peerpb.Message) {
	mirMsg := &pb.MirBFTMessage{}
	if err := proto.Unmarshal(message.Content, mirMsg); err != nil {
		panic(err)
	}

	if !mir.certifier.VerifyCertificate(message.Content, 0, message.Certificate) {
		panic("Invalid certificate")
	}

	switch t := mirMsg.Type.(type) {
	case *pb.MirBFTMessage_Agreement:
		mir.stepAgreement(t.Agreement, message.From)
	// case *pb.MirBFTMessage_EpochChange:
	// 	mir.onEpochChange(t.EpochChange, message.From)
	// case *pb.MirBFTMessage_EpochChangeAck:
	// 	mir.onEpochChangeAck(t.EpochChangeAck, message.From)
	// case *pb.MirBFTMessage_NewEpoch:
	// 	mir.onNewEpoch(t.NewEpoch, message.From)
	default:
		mir.logger.Panicf("Unknown message: %v", mirMsg)
	}
}

func (mir *MirBFT) stepAgreement(m *pb.AgreementMessage, from peerpb.PeerID) {
	if mir.viewChanging {
		mir.logger.Infof("View change in progress. Ignoring %v", m)
		return
	}

	// Check if this node is in the same view as the message
	if m.Epoch != mir.epoch {
		mir.logger.Errorf("Views do not match. Has: %v, Received: %v", mir.epoch, m.Epoch)
		return
	}

	mir.logger.Debugf("Replica %d ====[%v]====>>> Replica %d\n", from, m.Type, mir.id)

	switch m.Type {
	case pb.AgreementMessage_PrePrepare:
		mir.onPrePrepare(m, from)
	case pb.AgreementMessage_Prepare:
		mir.onPrepare(m, from)
	case pb.AgreementMessage_Commit:
		mir.onCommit(m, from)
	default:
		mir.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (mir *MirBFT) Tick() {
	for t := range mir.timers {
		t.Tick()
	}
}

var _ peer.Protocol = (*MirBFT)(nil)

func (p *MirBFT) hasPrePrepared(index pb.Index) bool {
	return p.hasStatus(index, pb.InstanceState_PrePrepared)
}

func (p *MirBFT) hasCommitted(index pb.Index) bool {
	return p.hasStatus(index, pb.InstanceState_Committed)
}

func (p *MirBFT) hasExecuted(index pb.Index) bool {
	return p.hasStatus(index, pb.InstanceState_Executed)
}

func (p *MirBFT) hasStatus(index pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := p.log[index]; ok {
		return inst.is.Status == status
	}
	return false
}
