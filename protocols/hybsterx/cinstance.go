package hybsterx

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

type instance struct {
	p      *hybsterx
	is     pb.InstanceState
	quorum *Quorum
}

func makeInstance(m *pb.NormalMessage, p *hybsterx) *instance {
	return &instance{
		p: p,
		is: pb.InstanceState{
			View:        m.View,
			InstanceID:  m.InstanceID,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		quorum: newQuorum(p),
	}
}

func (s *hybsterx) updateCommand(inst *instance, m *pb.NormalMessage) bool {
	if inst.is.Command != nil {
		if !inst.is.Command.Equal(m.Command) {
			s.logger.Panicf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.Command = m.Command
	}
	return true
}

func (s *hybsterx) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			s.logger.Panicf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (p *hybsterx) onPrepare(m *pb.NormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %v ===[Prepare %v]===>>> Replica %v\n", from, m,
		p.id)

	if p.isPrimaryAtView(p.id, p.oview) {
		p.onCRequest(m.InstanceID)
	}

	// update instance
	inst, exists := p.log[m.InstanceID]

	m.CommandHash = m.Command.Hash()

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.InstanceID] = inst
	} else if !p.updateCommand(inst, m) || !p.updateCommandHash(inst, m) {
		return
	}

	m.Command = nil
	inst.quorum.log(from, m)
	inst.is.Status = pb.InstanceState_Prepared

	cm := &pb.NormalMessage{
		View:        inst.is.View,
		InstanceID:  inst.is.InstanceID,
		Type:        pb.NormalMessage_Commit,
		CommandHash: inst.is.CommandHash,
	}
	p.broadcastNormal(cm)
	inst.quorum.log(p.id, cm)

	if inst.quorum.Majority(cm) && inst.is.IsPrepared() {
		inst.is.Status = pb.InstanceState_Committed
		p.exec()
	}
}

// HandleP2b handles P2b message
func (p *hybsterx) onCommit(m *pb.NormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %v ===[%v]===>>> Replica %v\n", from, m,
		p.id)

	inst, exists := p.log[m.InstanceID]

	// update instance
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.InstanceID] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.quorum.log(from, m)
	if inst.quorum.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.InstanceState_Committed
		p.exec()
	}
}

func (s *hybsterx) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(s.nodes))
}
