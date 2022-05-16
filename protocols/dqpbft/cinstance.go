package dqpbft

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
)

type instance struct {
	d *DQPBFT

	is pb.InstanceState

	pCert *Quorum
	cCert *Quorum
}

func makeInstance(m *pb.AgreementMessage, p *DQPBFT) *instance {
	return &instance{
		is: pb.InstanceState{
			View:        m.View,
			InstanceID:  m.InstanceID,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		pCert: newQuorum(p),
		cCert: newQuorum(p),
	}
}

func (d *DQPBFT) updateCommand(inst *instance, m *pb.AgreementMessage) bool {
	if inst.is.Command != nil {
		if !inst.is.Command.Equal(m.Command) {
			d.logger.Fatalf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.Command = m.Command
	}
	return true
}

func (d *DQPBFT) updateCommandHash(inst *instance, m *pb.AgreementMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			d.logger.Panicf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (d *DQPBFT) onPrePrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	if d.isPrimaryAtView(d.id, d.oview) {
		d.onCRequest(m.InstanceID)
	}

	hash := m.Command.Hash()
	m.CommandHash = hash

	inst, exists := d.log[m.InstanceID]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, d)
		d.log[m.InstanceID] = inst
	} else if !d.updateCommand(inst, m) || !d.updateCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	m.Command = nil
	inst.pCert.log(from, m)
	inst.is.Status = pb.InstanceState_PrePrepared
	pm := &pb.AgreementMessage{
		View:        inst.is.View,
		InstanceID:  inst.is.InstanceID,
		Type:        pb.AgreementMessage_Prepare,
		CommandHash: inst.is.CommandHash,
	}
	inst.pCert.log(d.id, pm)
	d.broadcast(pm, false)

	if inst.pCert.Majority(pm) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			View:        pm.View,
			InstanceID:  pm.InstanceID,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		d.broadcast(cm, false)

		inst.cCert.log(d.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			d.exec()
		}
	}
}

func (d *DQPBFT) onPrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	inst, exists := d.log[m.InstanceID]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, d)
		d.log[m.InstanceID] = inst
	} else if !d.updateCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			View:        m.View,
			InstanceID:  m.InstanceID,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		d.broadcast(cm, false)

		inst.cCert.log(d.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			d.exec()
		}
	}
}

// HandleP2b handles P2b message
func (d *DQPBFT) onCommit(m *pb.AgreementMessage, from peerpb.PeerID) {
	inst, exists := d.log[m.InstanceID]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, d)
		d.log[m.InstanceID] = inst
	} else if !d.updateCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m)
	if inst.cCert.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.InstanceState_Committed
		d.exec()
	}
	//p.logger.Debugf("commit acks for %v: %v", m.Command.ID, len(p.log[m.Index].cCert.msgs))
}
