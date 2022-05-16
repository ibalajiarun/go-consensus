package dispel

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
)

type rbroadcast struct {
	rbs        pb.RBState
	pCert      *quorum
	cCert      *quorum
	reproposed bool
	epoch      *epoch
}

func (d *Dispel) makeRB(ep *epoch, m *pb.RBMessage) *rbroadcast {
	return &rbroadcast{
		rbs: pb.RBState{
			PeerID:      m.PeerID,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		pCert: newQuorum(d),
		cCert: newQuorum(d),

		epoch: ep,
	}
}

func (d *Dispel) updateCommand(ep *epoch, m *pb.RBMessage) bool {
	rb, ok := ep.rbs[m.PeerID]
	if !ok {
		rb = d.makeRB(ep, m)
		ep.rbs[m.PeerID] = rb
	} else if rb.rbs.Command != nil {
		if !rb.rbs.Command.Equal(m.Command) {
			d.logger.Panicf("Different commands for same instance %v: Received: %v, Has %v.", ep, rb.rbs.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		rb.rbs.Command = m.Command
	}

	return true
}

func (d *Dispel) updateCommandHash(ep *epoch, m *pb.RBMessage) bool {
	rb, ok := ep.rbs[m.PeerID]
	if !ok {
		rb = d.makeRB(ep, m)
		ep.rbs[m.PeerID] = rb
	} else if rb.rbs.CommandHash != nil {
		if !bytes.Equal(rb.rbs.CommandHash, m.CommandHash) {
			d.logger.Panicf("Different command hashes for same instance %v: Received: %x, Has %x.", ep, rb.rbs.CommandHash, m.CommandHash)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		rb.rbs.CommandHash = m.CommandHash
	}
	return true
}

func (d *Dispel) onRBSend(m *pb.RBMessage, from peerpb.PeerID) {
	m.CommandHash = m.Command.Hash()

	ep, exists := d.epochs[m.EpochNum]
	if !exists {
		ep = makeEpoch(m, d)
		ep.rbs[m.PeerID] = d.makeRB(ep, m)
		d.epochs[m.EpochNum] = ep
	} else if !d.updateCommand(ep, m) || !d.updateCommandHash(ep, m) {
		return
	}

	rb, exists := ep.rbs[m.PeerID]
	if !exists {
		d.logger.Panicf("slot not available %v", m)
	}

	m.Command = nil
	rb.pCert.log(from, m)
	rb.rbs.Status = pb.RBState_Received
	pm := &pb.RBMessage{
		EpochNum:    m.EpochNum,
		PeerID:      m.PeerID,
		Type:        pb.RBMessage_Echo,
		CommandHash: rb.rbs.CommandHash,
	}
	rb.pCert.log(d.id, pm)
	d.broadcast(pm, false)

	if rb.pCert.Majority(pm) && rb.rbs.HasReceived() {
		rb.rbs.Status = pb.RBState_Echoed
		cm := &pb.RBMessage{
			EpochNum:    m.EpochNum,
			PeerID:      m.PeerID,
			Type:        pb.RBMessage_Ready,
			CommandHash: rb.rbs.CommandHash,
		}
		d.broadcast(cm, false)

		rb.cCert.log(d.id, cm)
		if rb.cCert.Majority(cm) && rb.rbs.HasEchoed() {
			rb.rbs.Status = pb.RBState_Readied
			ep.rbDoneCount++
			d.tryStartConsensus(ep)
			d.onRBDone(ep, rb.rbs.PeerID)
			d.exec()
			d.onAfterReady(rb)
		}
	}
}

func (d *Dispel) onRBEcho(m *pb.RBMessage, from peerpb.PeerID) {
	ep, exists := d.epochs[m.EpochNum]
	if !exists {
		ep = makeEpoch(m, d)
		ep.rbs[m.PeerID] = d.makeRB(ep, m)
		d.epochs[m.EpochNum] = ep
	} else if !d.updateCommandHash(ep, m) {
		return
	}

	rb, exists := ep.rbs[m.PeerID]
	if !exists {
		d.logger.Panicf("slot not available %v", m)
	}

	rb.pCert.log(from, m)
	if rb.pCert.Majority(m) && rb.rbs.HasReceived() {
		rb.rbs.Status = pb.RBState_Echoed
		cm := &pb.RBMessage{
			EpochNum:    m.EpochNum,
			PeerID:      m.PeerID,
			Type:        pb.RBMessage_Ready,
			CommandHash: rb.rbs.CommandHash,
		}
		d.broadcast(cm, false)

		rb.cCert.log(d.id, cm)
		if rb.cCert.Majority(cm) && rb.rbs.HasEchoed() {
			rb.rbs.Status = pb.RBState_Readied
			ep.rbDoneCount++
			d.tryStartConsensus(ep)
			d.onRBDone(ep, rb.rbs.PeerID)
			d.exec()
			d.onAfterReady(rb)
		}
	}
}

// onRBReady handles RBReady message
func (d *Dispel) onRBReady(m *pb.RBMessage, from peerpb.PeerID) {
	ep, exists := d.epochs[m.EpochNum]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		ep = makeEpoch(m, d)
		ep.rbs[m.PeerID] = d.makeRB(ep, m)
		d.epochs[m.EpochNum] = ep
	} else if !d.updateCommandHash(ep, m) {
		return
	}

	rb, exists := ep.rbs[m.PeerID]
	if !exists {
		d.logger.Panicf("slot not available %v", m)
	}

	rb.cCert.log(from, m)
	if rb.cCert.Majority(m) && rb.rbs.HasEchoed() {
		rb.rbs.Status = pb.RBState_Readied
		ep.rbDoneCount++
		d.tryStartConsensus(ep)
		d.onRBDone(ep, rb.rbs.PeerID)
		d.exec()
		d.onAfterReady(rb)
	}
}

func (d *Dispel) onAfterReady(rb *rbroadcast) {
	if d.id == rb.rbs.PeerID {
		if cons, ok := rb.epoch.cons[rb.rbs.PeerID]; ok &&
			cons.decided && cons.decidedValue == 0 {
			if !rb.reproposed {
				d.onRequest(rb.rbs.Command)
				promReproposeCount.Inc()
			}
		}
	}
}
