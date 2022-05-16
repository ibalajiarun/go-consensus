package rcc

import (
	"container/list"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/protocols/pbft"
	pb "github.com/ibalajiarun/go-consensus/protocols/rcc/rccpb"
	"github.com/ibalajiarun/go-consensus/protocols/sbft"
)

type RCC struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	replicators map[peerpb.PeerID]peer.Protocol

	execPkts              map[peerpb.PeerID]*list.List
	execIdx               int
	globalOrderedCommands []peer.ExecPacket
	// execPktsWithoutCb     map[peerpb.PeerID][]peer.ExecPacket
	// execIdxCb             int

	logger logger.Logger
}

func NewRCC(cfg *peer.LocalConfig) *RCC {
	p := &RCC{
		id:    cfg.ID,
		nodes: cfg.Peers,

		replicators: make(map[peerpb.PeerID]peer.Protocol, len(cfg.Peers)),
		execPkts:    make(map[peerpb.PeerID]*list.List),
		// execPktsWithCb:    make(map[peerpb.PeerID][]peer.ExecPacket, len(cfg.Peers)),
		// execPktsWithoutCb: make(map[peerpb.PeerID][]peer.ExecPacket, len(cfg.Peers)),

		logger: cfg.Logger,
	}

	for _, id := range cfg.Peers {
		rpCfg := *cfg
		pCfg := *rpCfg.PeerConfig
		rpCfg.PeerConfig = &pCfg
		rpCfg.LeaderID = id

		var rp peer.Protocol
		switch cfg.RccAlgorithm {
		case peerpb.Algorithm_PBFT:
			rp = pbft.NewPBFT(&rpCfg)
		case peerpb.Algorithm_SBFT:
			rp = sbft.NewSBFT(&rpCfg)
		default:
			cfg.Logger.Panicf("Unknown protocol specified %v", cfg.Algorithm)
		}
		p.replicators[id] = rp

		p.execPkts[id] = list.New()
	}

	return p
}

func (d *RCC) AsyncCallback() {
	for _, r := range d.replicators {
		r.AsyncCallback()
	}
}

func (d *RCC) Callback(cb peer.ExecCallback) {
	d.replicators[cb.Cmd.Target].Callback(cb)
}

func (d *RCC) ClearExecutedCommands() {
	d.globalOrderedCommands = nil
}

func (d *RCC) ClearMsgs() {
	for _, r := range d.replicators {
		r.ClearMsgs()
	}
}

func (d *RCC) MakeReady() peer.Ready {
	var messages []peerpb.Message
	for id, r := range d.replicators {
		rReady := r.MakeReady()
		for _, msg := range rReady.Messages {
			dm := &pb.RCCMessage{
				PeerID:  id,
				Message: msg,
			}
			mBytes, err := proto.Marshal(dm)
			if err != nil {
				panic(err)
			}
			messages = append(messages, peerpb.Message{
				From:    msg.From,
				To:      msg.To,
				Content: mBytes,
			})
		}
		for _, pkt := range rReady.OrderedCommands {
			if !pkt.NoExec {
				d.execPkts[id].PushBack(pkt)
			} else {
				d.globalOrderedCommands = append(d.globalOrderedCommands, pkt)
			}
		}
		r.ClearExecutedCommands()
	}

	present := true
	for _, pkts := range d.execPkts {
		if pkts.Len() == 0 {
			present = false
			break
		}
	}

	if present {
		for _, list := range d.execPkts {
			elem := list.Front()
			if elem != nil {
				val := list.Remove(elem)
				d.globalOrderedCommands = append(d.globalOrderedCommands, val.(peer.ExecPacket))
			}
		}
	}

	return peer.Ready{
		Messages:        messages,
		OrderedCommands: d.globalOrderedCommands,
	}
}

func (d *RCC) Request(r *commandpb.Command) {
	d.replicators[r.Target].Request(r)
}

func (d *RCC) Step(m peerpb.Message) {
	dm := pb.RCCMessage{}
	err := proto.Unmarshal(m.Content, &dm)
	if err != nil {
		d.logger.Panicf("Unable to unmarshall %v", err)
	}

	d.replicators[dm.PeerID].Step(dm.Message)
}

func (d *RCC) Tick() {
	for _, r := range d.replicators {
		r.Tick()
	}
}

func (d *RCC) hasExecuted(cmd *commandpb.Command) bool {
	for _, pkt := range d.globalOrderedCommands {
		if pkt.Cmd.Equal(cmd) {
			return true
		}
	}
	return false
}

var _ peer.Protocol = (*RCC)(nil)
