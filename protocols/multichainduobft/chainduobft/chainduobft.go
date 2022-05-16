package chainduobft

import (
	"container/list"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/usig"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
	"github.com/oasisprotocol/ed25519"
)

type ChainDuoBFT struct {
	id          peerpb.PeerID
	peers       []peerpb.PeerID
	maxFailures int

	logger logger.Logger

	cmdQueue      *list.List
	msgs          []peerpb.Message
	committedCmds []peer.ExecPacket

	blockchain *blockchain
	pacemaker  *pacemaker

	lastVoteFastHeight        pb.Height
	lastVoteSlowHeight        pb.Height
	lastFastHeightWithCommand pb.Height
	lastFastHeight            pb.Height
	lastSlowHeightWithCommand pb.Height
	lastSlowHeight            pb.Height

	bLockFast *block
	bExecFast *block

	bLockSlow *block
	bExecSlow *block

	verifiedVotes        map[pb.BlockHash][]pb.VoteMessage
	pendingFastProposals map[pb.BlockHash]*pb.ProposeMessage
	pendingSlowProposals map[pb.BlockHash]*pb.ProposeMessage
	waitQueueForHeight   map[pb.Height]func()

	fastQuorumSize int
	slowQuorumSize int
	skipSlowPath   bool

	signer               *signer
	signUsigDispatcher   *worker.Dispatcher
	signDispatcher       *worker.Dispatcher
	fastVerifyDispatcher *worker.Dispatcher
	slowVerifyDispatcher *worker.Dispatcher
	callbackC            chan func()

	timers map[peer.TickingTimer]struct{}
}

func NewChainDuoBFTWithEnclave(
	cfg *peer.LocalConfig, initialView pb.View,
	enclave *usig.UsigEnclave, ctrID int,
	usigDispatcher, signDispatcher, fastVerifyDispatcher, slowVerifyDispatcher *worker.Dispatcher,
	pubKey ed25519.PublicKey, privKey ed25519.PrivateKey,
) *ChainDuoBFT {
	fastQuorumSize := int(cfg.MaxFailures) + 1
	slowQuorumSize := 2*int(cfg.MaxFailures) + 1

	d := &ChainDuoBFT{
		id:          cfg.ID,
		peers:       cfg.Peers,
		maxFailures: int(cfg.MaxFailures),

		logger: cfg.Logger,

		cmdQueue: list.New(),

		blockchain: newBlockchain(),

		bLockFast: genesis,
		bExecFast: genesis,

		bLockSlow: genesis,
		bExecSlow: genesis,

		verifiedVotes:        make(map[pb.BlockHash][]pb.VoteMessage),
		pendingFastProposals: make(map[pb.BlockHash]*pb.ProposeMessage),
		pendingSlowProposals: make(map[pb.BlockHash]*pb.ProposeMessage),
		waitQueueForHeight:   make(map[pb.Height]func()),

		fastQuorumSize: fastQuorumSize,
		slowQuorumSize: slowQuorumSize,
		skipSlowPath:   cfg.MultiChainDuoBFT.SkipSlowPath,

		signer: newSigner(ctrID, enclave, privKey, pubKey),

		timers: make(map[peer.TickingTimer]struct{}),
	}

	d.pacemaker = newPacemaker(d, initialView)

	totalWorkers := 64
	if signDispatcher == nil || fastVerifyDispatcher == nil || slowVerifyDispatcher == nil {
		keys := []string{"edd_sign", "edd_verify"}
		for _, k := range keys {
			if _, ok := cfg.Workers[k]; !ok {
				d.logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
			}
			if count, ok := cfg.WorkersQueueSizes[k]; !ok {
				d.logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
			} else {
				totalWorkers += int(count)
			}
		}
	}
	d.callbackC = make(chan func(), totalWorkers)

	if usigDispatcher != nil {
		d.signUsigDispatcher = usigDispatcher
	} else {
		d.signUsigDispatcher = worker.NewDispatcher(
			int(cfg.WorkersQueueSizes["edd_sign"]),
			1,
			d.callbackC,
		)
		d.signUsigDispatcher.Run()
	}

	if signDispatcher != nil {
		d.signDispatcher = signDispatcher
	} else {
		d.signDispatcher = worker.NewDispatcher(
			int(cfg.WorkersQueueSizes["edd_sign"]),
			int(cfg.Workers["edd_sign"]),
			d.callbackC,
		)
		d.signDispatcher.Run()
	}

	if fastVerifyDispatcher != nil {
		d.fastVerifyDispatcher = fastVerifyDispatcher
	} else {
		d.fastVerifyDispatcher = worker.NewDispatcher(
			int(cfg.WorkersQueueSizes["edd_verify"]),
			int(cfg.Workers["edd_verify"]),
			d.callbackC,
		)
		d.fastVerifyDispatcher.Run()
	}

	if slowVerifyDispatcher != nil {
		d.slowVerifyDispatcher = slowVerifyDispatcher
	} else {
		d.slowVerifyDispatcher = worker.NewDispatcher(
			int(cfg.WorkersQueueSizes["edd_verify"]),
			int(cfg.Workers["edd_verify"]),
			d.callbackC,
		)
		d.slowVerifyDispatcher.Run()
	}

	// d.initTimers()

	d.logger.Infof("Skip Slow Path: %v", d.skipSlowPath)

	return d
}

func NewChainDuoBFT(cfg *peer.LocalConfig) *ChainDuoBFT {
	enclave, pubKey, privKey := MakeUsigSignerEnclave(cfg.EnclavePath, 1)

	return NewChainDuoBFTWithEnclave(cfg, pb.View(cfg.LeaderID), enclave, 0,
		nil, nil, nil, nil, pubKey, privKey)
}

func (d *ChainDuoBFT) Tick() {
	for t := range d.timers {
		t.Tick()
	}
}

func (d *ChainDuoBFT) Request(cmd *commandpb.Command) {
	d.onRequest(cmd)
}

func (d *ChainDuoBFT) Step(message peerpb.Message) {
	dMsg := &pb.ChainDuoBFTMessage{}
	if err := proto.Unmarshal(message.Content, dMsg); err != nil {
		panic(err)
	}

	switch t := dMsg.Type.(type) {
	case *pb.ChainDuoBFTMessage_Propose:
		d.onPropose(t.Propose, message.From)
	case *pb.ChainDuoBFTMessage_Vote:
		d.onVote(t.Vote)
	default:
		panic("Unknown message type")
	}
}

func (d *ChainDuoBFT) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        d.msgs,
		OrderedCommands: d.committedCmds,
	}
}

func (d *ChainDuoBFT) AsyncCallback() {
	for {
		select {
		case cb := <-d.callbackC:
			cb()
		default:
			return
		}
	}
}

func (*ChainDuoBFT) Callback(peer.ExecCallback) {

}

func (d *ChainDuoBFT) ClearExecutedCommands() {
	d.committedCmds = nil
}

func (d *ChainDuoBFT) enqueueForExecution(cmd *commandpb.Command) {
	d.committedCmds = append(d.committedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (d *ChainDuoBFT) scheduleBack(step func()) {
	select {
	case d.callbackC <- step:
	default:
		d.logger.Panicf("could not schedule callback: channel size: %d", len(d.callbackC))
	}
}

func (d *ChainDuoBFT) hasExecuted(cmd *commandpb.Command) bool {
	for i := range d.committedCmds {
		if d.committedCmds[i].Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}

func (d *ChainDuoBFT) HasFastExecuted(cmd *commandpb.Command) bool {
	for _, blk := range d.blockchain.fastBlocks {
		if blk.FastState != nil && blk.FastState.Command != nil && blk.FastState.Command.Equal(cmd) {
			if d.bExecFast.FastState.Height >= blk.FastState.Height {
				return true
			}
		}
	}
	return false
}

func (d *ChainDuoBFT) initTimers() {

}

func (d *ChainDuoBFT) registerInfiniteTimer(t peer.TickingTimer) {
	d.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (d *ChainDuoBFT) registerOneTimeTimer(t peer.TickingTimer) {
	d.timers[t] = struct{}{}
	t.Instrument(func() {
		d.unregisterTimer(t)
	})
	t.Reset()
}

func (d *ChainDuoBFT) unregisterTimer(t peer.TickingTimer) {
	t.Stop()
	delete(d.timers, t)
}

var _ peer.Protocol = ((*ChainDuoBFT)(nil))
