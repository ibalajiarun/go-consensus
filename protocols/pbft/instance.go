package pbft

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
)

type instance struct {
	p *PBFT

	is pb.InstanceState

	pCert *Quorum
	cCert *Quorum
}

func makeInstance(m *pb.AgreementMessage, p *PBFT) *instance {
	return &instance{
		is: pb.InstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		pCert: newQuorum(p),
		cCert: newQuorum(p),
	}
}

func (s *PBFT) updateCommand(inst *instance, m *pb.AgreementMessage) bool {
	if inst.is.Command != nil {
		if !inst.is.Command.Equal(m.Command) {
			s.logger.Fatalf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.Command = m.Command
	}
	return true
}

func (s *PBFT) updateCommandHash(inst *instance, m *pb.AgreementMessage) bool {
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

func (s *PBFT) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	compHash := cmd.Hash()
	return bytes.Equal(hash, compHash[:])
}

func (p *PBFT) onPrePrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	// Check if the message sender is the current primary
	if !p.isCurrentPrimary(from) {
		p.logger.Errorf("PrePrepare vcSender Non-Primary %v: %v", from, m)
		return
	}

	if !p.checkCommandHash(m.Command, m.CommandHash) {
		p.logger.Panicf("The hash does not correspond to the command: %v, %v", m, m.CommandHash)
	}

	inst, exists := p.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Index] = inst
	} else if !p.updateCommand(inst, m) || !p.updateCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	m.Command = nil
	inst.pCert.log(from, m)
	inst.is.Status = pb.InstanceState_PrePrepared
	pm := &pb.AgreementMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.AgreementMessage_Prepare,
		CommandHash: inst.is.CommandHash,
	}
	inst.pCert.log(p.id, pm)
	p.broadcast(pm, false)
	if inst.pCert.Majority(pm) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			View:        pm.View,
			Index:       pm.Index,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		p.broadcast(cm, false)

		inst.cCert.log(p.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			p.exec()
		}
	}
}

func (p *PBFT) onPrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	if p.isCurrentPrimary(from) {
		p.logger.Errorf("Prepare received vcSender Primary %v: %v", from, m)
		return
	}

	inst, exists := p.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Index] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			View:        m.View,
			Index:       m.Index,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		p.broadcast(cm, false)

		inst.cCert.log(p.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			p.exec()
		}
	}
}

// HandleP2b handles P2b message
func (p *PBFT) onCommit(m *pb.AgreementMessage, from peerpb.PeerID) {
	inst, exists := p.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Index] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m)
	if inst.cCert.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.InstanceState_Committed
		p.exec()
	}
	//p.logger.Debugf("commit acks for %v: %v", m.Command.ID, len(p.log[m.Index].cCert.msgs))
}

func (p *PBFT) exec() {
	for {
		inst, ok := p.log[p.execute]
		if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
			break
		}

		p.logger.Debugf("Replica %d execute [s=%d, cmd=%d]\n", p.id, p.execute,
			inst.is.Command.Timestamp)

		p.Execute(inst.is.Command)
		inst.is.Command = nil
		p.execute++
	}
}

func (p *PBFT) isCurrentPrimary(id peerpb.PeerID) bool {
	return p.isPrimaryAtView(id, p.view)
}

func (p *PBFT) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(p.nodes))
}
