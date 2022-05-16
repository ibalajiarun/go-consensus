package chainhotstuff

import (
	"container/list"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

type chainhotstuff struct {
	id    peerpb.PeerID
	peers []peerpb.PeerID

	msgs          []peerpb.Message
	committedCmds []peer.ExecPacket

	timers map[peer.TickingTimer]struct{}
	logger logger.Logger
	f      int

	cmdQueue  *list.List
	pacemaker *pacemaker
	signer    *signer

	blocks map[pb.BlockHash]*block

	lastVote pb.View        // the last view that the replica voted in
	bLock    *block         // the currently locked block
	bExec    *block         // the last committed block
	bLeaf    *block         // the last proposed block
	highQC   *pb.QuorumCert // the highest qc known to this replica

	verifiedVotes    map[pb.BlockHash][]pb.VoteMessage // verified votes that could become a QC
	pendingVotes     map[pb.BlockHash][]pb.VoteMessage // unverified votes that are waiting for a Block
	pendingProposals map[pb.BlockHash]*pb.ProposeMessage
	newView          map[pb.View]map[peerpb.PeerID]struct{} // the set of replicas who have sent a newView message per view

	verifyDispatcher *worker.Dispatcher
	callbackC        chan func()
}

func NewChainHotstuff(cfg *peer.LocalConfig) peer.Protocol {
	quorumSize := 2*int(cfg.MaxFailures) + 1
	hs := &chainhotstuff{
		id:    cfg.ID,
		peers: cfg.Peers,

		timers: make(map[peer.TickingTimer]struct{}),
		logger: cfg.Logger,
		f:      int(cfg.MaxFailures),

		cmdQueue: list.New(),
		signer:   newSigner(quorumSize),

		blocks: make(map[pb.BlockHash]*block, 65536),

		bLock: genesis,
		bExec: genesis,
		bLeaf: genesis,

		verifiedVotes:    make(map[pb.BlockHash][]pb.VoteMessage),
		pendingVotes:     make(map[pb.BlockHash][]pb.VoteMessage),
		pendingProposals: make(map[pb.BlockHash]*pb.ProposeMessage),
		newView:          make(map[pb.View]map[peerpb.PeerID]struct{}),
	}
	hs.pacemaker = newPacemaker(hs)

	var err error
	hs.highQC, err = hs.signer.createQuorumCert(genesis, []pb.VoteMessage{})
	if err != nil {
		hs.logger.Panicf("Failed to create QC for genesis block!")
	}
	hs.blocks[genesis.hash()] = genesis

	if _, ok := cfg.Workers["edd_verify"]; !ok {
		hs.logger.Panicf("Worker count for edd_verify missing.")
	}
	if _, ok := cfg.WorkersQueueSizes["edd_verify"]; !ok {
		hs.logger.Panicf("Worker queue size for edd_verify missing.")
	}

	hs.callbackC = make(chan func(), cfg.WorkersQueueSizes["edd_verify"])
	hs.verifyDispatcher = worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["edd_verify"]),
		int(cfg.Workers["edd_verify"]),
		hs.callbackC,
	)
	hs.verifyDispatcher.Run()

	if hs.id == hs.pacemaker.getLeader(hs.bLeaf.Height+1) {
		hs.propose()
	}
	return hs
}

func (hs *chainhotstuff) Tick() {
	for t := range hs.timers {
		t.Tick()
	}
}

func (hs *chainhotstuff) registerInfiniteTimer(t peer.TickingTimer) {
	hs.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (hs *chainhotstuff) Request(cmd *commandpb.Command) {
	hs.onRequest(cmd)
}

func (hs *chainhotstuff) Step(message peerpb.Message) {
	hMsg := &pb.ChainHotstuffMessage{}
	if err := proto.Unmarshal(message.Content, hMsg); err != nil {
		panic(err)
	}

	switch t := hMsg.Type.(type) {
	case *pb.ChainHotstuffMessage_Propose:
		hs.onPropose(t.Propose)
	case *pb.ChainHotstuffMessage_Vote:
		hs.onVote(t.Vote)
	case *pb.ChainHotstuffMessage_NewView:
		hs.onNewView(message.From, t.NewView)
	default:
		panic("Unknown message type")
	}
}

func (hs *chainhotstuff) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        hs.msgs,
		OrderedCommands: hs.committedCmds,
	}
}

func (hs *chainhotstuff) execute(cmd *commandpb.Command) {
	hs.committedCmds = append(hs.committedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (hs *chainhotstuff) ClearExecutedCommands() {
	hs.committedCmds = nil
}

func (*chainhotstuff) Callback(peer.ExecCallback) {

}

func (hs *chainhotstuff) AsyncCallback() {
	for {
		select {
		case cb := <-hs.callbackC:
			cb()
		default:
			return
		}
	}
}

func (hs *chainhotstuff) hasExecuted(cmd *commandpb.Command) bool {
	for i := range hs.committedCmds {
		if hs.committedCmds[i].Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}

func (hs *chainhotstuff) hasSlowExecuted(cmd *commandpb.Command) bool {
	for i := range hs.committedCmds {
		if hs.committedCmds[i].Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}
