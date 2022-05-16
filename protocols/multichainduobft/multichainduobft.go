package multichainduobft

import (
	"container/list"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	"github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft"
	"github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/multichainduobftpb"
)

type multichainduobft struct {
	id    peerpb.PeerID
	peers []peerpb.PeerID

	chains map[pb.ChainID]*chainduobft.ChainDuoBFT
	logger logger.Logger

	seq              int
	numChainsPerPeer int

	globalOrderedCommands []peer.ExecPacket
	execPkts              map[pb.ChainID]*list.List
	chainIDBytes          map[pb.ChainID][]byte
	messages              []peerpb.Message

	callbackC chan func()
}

func NewMultiChainDuoBFT(cfg *peer.LocalConfig) *multichainduobft {
	logger := cfg.Logger

	numChainsPerPeer := int(cfg.MultiChainDuoBFT.InstancesPerPeer)
	numCounters := numChainsPerPeer * len(cfg.Peers)
	numChains := numChainsPerPeer
	if cfg.MultiChainDuoBFT.RCCMode {
		numChains = numChains * len(cfg.Peers)
	}

	enclave, pubKey, privKey := chainduobft.MakeUsigSignerEnclave(cfg.EnclavePath, int(numCounters))

	keys := []string{"mcd_edd_usig_sign", "mcd_edd_sign", "mcd_edd_verify"}
	totalWorkers := 0
	for _, k := range keys {
		if _, ok := cfg.Workers[k]; !ok {
			logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
		}
		if count, ok := cfg.WorkersQueueSizes[k]; !ok {
			logger.Panicf("Worker count for key missing: %s. Required: %v", k, keys)
		} else {
			totalWorkers += int(count)
		}
	}

	callbackC := make(chan func(), totalWorkers)
	usigDispatcher := worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mcd_edd_usig_sign"]),
		int(cfg.Workers["mcd_edd_usig_sign"]),
		callbackC,
	)
	usigDispatcher.Run()

	signDispatcher := worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mcd_edd_sign"]),
		int(cfg.Workers["mcd_edd_sign"]),
		callbackC,
	)
	signDispatcher.Run()

	fastVerifyDispatcher := worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mcd_edd_verify"]),
		int(cfg.Workers["mcd_edd_verify"]),
		callbackC,
	)
	fastVerifyDispatcher.Run()

	slowVerifyDispatcher := worker.NewDispatcher(
		int(cfg.WorkersQueueSizes["mcd_edd_verify"]),
		int(cfg.Workers["mcd_edd_verify"]),
		callbackC,
	)
	slowVerifyDispatcher.Run()

	chains := make(map[pb.ChainID]*chainduobft.ChainDuoBFT, numChains)
	execPkts := make(map[pb.ChainID]*list.List)
	for peerID := range cfg.Peers {
		if !cfg.MultiChainDuoBFT.RCCMode && peerID != int(cfg.LeaderID) {
			continue
		}
		for seq := uint32(0); seq < cfg.MultiChainDuoBFT.InstancesPerPeer; seq++ {
			chainID := pb.ChainID{
				PeerID:  peerpb.PeerID(peerID),
				PeerSeq: seq,
			}
			ctrID := chainIDToCounterID(chainID, numChainsPerPeer)
			chains[chainID] = chainduobft.NewChainDuoBFTWithEnclave(
				cfg, chainduobftpb.View(peerID),
				enclave, ctrID,
				usigDispatcher, signDispatcher,
				fastVerifyDispatcher, slowVerifyDispatcher,
				pubKey, privKey,
			)
			execPkts[chainID] = list.New()
		}
	}

	chainIDBytes := make(map[pb.ChainID][]byte)
	for id := range chains {
		idBytes, err := proto.Marshal(&id)
		if err != nil {
			panic(err)
		}
		chainIDBytes[id] = idBytes
	}

	mcduobft := &multichainduobft{
		id:    cfg.ID,
		peers: cfg.Peers,

		chains: chains,
		logger: cfg.Logger,

		numChainsPerPeer: numChainsPerPeer,
		execPkts:         execPkts,
		chainIDBytes:     chainIDBytes,

		callbackC: callbackC,
	}

	return mcduobft
}

func (md *multichainduobft) Tick() {
	for _, d := range md.chains {
		d.Tick()
	}
}

func (md *multichainduobft) Request(cmd *commandpb.Command) {
	cid := pb.ChainID{
		PeerID:  cmd.Target,
		PeerSeq: uint32(md.seq % md.numChainsPerPeer),
	}
	md.seq = md.seq + 1
	md.chains[cid].Request(cmd)
}

func (md *multichainduobft) Step(m peerpb.Message) {
	var chainID pb.ChainID
	err := proto.Unmarshal(m.Certificate, &chainID)
	if err != nil {
		md.logger.Panicf("Unable to unmarshall %v", err)
	}
	d, ok := md.chains[chainID]
	if !ok {
		md.logger.Panicf("chain not available: %v", chainID)
	}
	d.Step(m)
}

func (md *multichainduobft) MakeReady() peer.Ready {
	for id, r := range md.chains {
		rReady := r.MakeReady()
		for _, msg := range rReady.Messages {
			msg.Certificate = md.chainIDBytes[id]
			md.messages = append(md.messages, msg)
		}
		r.ClearMsgs()

		for _, pkt := range rReady.OrderedCommands {
			if !pkt.NoExec {
				md.execPkts[id].PushBack(pkt)
			} else {
				md.globalOrderedCommands = append(md.globalOrderedCommands, pkt)
			}
		}
		r.ClearExecutedCommands()
	}

	present := true
	for _, pkts := range md.execPkts {
		if pkts.Len() == 0 {
			present = false
			break
		}
	}

	if present {
		for _, list := range md.execPkts {
			elem := list.Front()
			val := list.Remove(elem)
			md.globalOrderedCommands = append(md.globalOrderedCommands, val.(peer.ExecPacket))
		}
	}

	return peer.Ready{
		Messages:        md.messages,
		OrderedCommands: md.globalOrderedCommands,
	}
}

func (md *multichainduobft) AsyncCallback() {
	for _, d := range md.chains {
		d.AsyncCallback()
	}
	for {
		select {
		case cb := <-md.callbackC:
			cb()
		default:
			return
		}
	}
}

func (md *multichainduobft) Callback(peer.ExecCallback) {
}

func (md *multichainduobft) ClearExecutedCommands() {
	md.globalOrderedCommands = nil
}

func (md *multichainduobft) ClearMsgs() {
	md.messages = nil
}

func (md *multichainduobft) hasExecuted(cmd *commandpb.Command) bool {
	for _, pkt := range md.globalOrderedCommands {
		if pkt.Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}

func (md *multichainduobft) hasFastExecuted(cmd *commandpb.Command) bool {
	for _, d := range md.chains {
		if d.HasFastExecuted(cmd) {
			return true
		}
	}
	return false
}

var _ peer.Protocol = (*multichainduobft)(nil)

func chainIDToCounterID(chainID pb.ChainID, multiplier int) int {
	return int(chainID.PeerID)*multiplier + int(chainID.PeerSeq)
}
