package dispel

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
)

func (d *Dispel) onRequest(cmd *commandpb.Command) *epoch {
	epochNumber := d.nextEpochNumber
	d.nextEpochNumber++

	ep, ok := d.epochs[epochNumber]
	if !ok {
		ep = &epoch{
			epochNum: epochNumber,

			rbs:  make(map[peerpb.PeerID]*rbroadcast),
			cons: make(map[peerpb.PeerID]*dbft),

			d: d,
		}
		d.epochs[epochNumber] = ep
	}

	rb, ok := ep.rbs[d.id]
	if !ok {
		rb = &rbroadcast{
			rbs: pb.RBState{
				PeerID:      d.id,
				Command:     cmd,
				CommandHash: cmd.Hash(),
			},
			pCert: newQuorum(d),
			cCert: newQuorum(d),

			epoch: ep,
		}
		ep.rbs[d.id] = rb
	}

	pm := &pb.RBMessage{
		EpochNum: ep.epochNum,
		PeerID:   rb.rbs.PeerID,
		Type:     pb.RBMessage_Send,
		Command:  cmd,
	}
	logpm := &pb.RBMessage{
		EpochNum:    ep.epochNum,
		PeerID:      rb.rbs.PeerID,
		Type:        pb.RBMessage_Send,
		CommandHash: cmd.Hash(),
	}
	rb.rbs.Status = pb.RBState_Received
	rb.pCert.log(d.id, logpm)
	ep.d.broadcast(pm, false)

	return ep
}
