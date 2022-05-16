package mirbft

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/mirbft/mirbftpb"
)

type instance struct {
	mir *MirBFT

	is pb.InstanceState

	pCert *Quorum
	cCert *Quorum
}

func makeInstance(m *pb.AgreementMessage, mir *MirBFT) *instance {
	return &instance{
		is: pb.InstanceState{
			Epoch:       m.Epoch,
			Index:       m.Index,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		pCert: newQuorum(mir),
		cCert: newQuorum(mir),
	}
}

func (mir *MirBFT) updateCommand(inst *instance, m *pb.AgreementMessage) bool {
	if inst.is.Command != nil {
		if !inst.is.Command.Equal(m.Command) {
			mir.logger.Fatalf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.Command = m.Command
	}
	return true
}

func (mir *MirBFT) updateCommandHash(inst *instance, m *pb.AgreementMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			mir.logger.Panicf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (mir *MirBFT) onPrePrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	// Check if the message sender is the current leader
	if _, ok := mir.epochLeaders[m.Epoch][from]; !ok {
		mir.logger.Errorf("PrePrepare vcSender Non-leader %v: %v", from, m)
		return
	}

	if mir.leaderOfSeq(m.Epoch, m.Index) != from {
		mir.logger.Errorf("Not the leader %v: %v", from, m)
		return
	}

	hash := m.Command.Hash()
	m.CommandHash = hash

	inst, exists := mir.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, mir)
		mir.log[m.Index] = inst
	} else if !mir.updateCommand(inst, m) || !mir.updateCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	m.Command = nil
	inst.pCert.log(from, m)
	inst.is.Status = pb.InstanceState_PrePrepared
	pm := &pb.AgreementMessage{
		Epoch:       inst.is.Epoch,
		Index:       inst.is.Index,
		Type:        pb.AgreementMessage_Prepare,
		CommandHash: inst.is.CommandHash,
	}
	inst.pCert.log(mir.id, pm)
	mir.broadcast(pm, false)
	if inst.pCert.Majority(pm) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			Epoch:       pm.Epoch,
			Index:       pm.Index,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		mir.broadcast(cm, false)

		inst.cCert.log(mir.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			mir.exec()
		}
	}
}

func (mir *MirBFT) onPrepare(m *pb.AgreementMessage, from peerpb.PeerID) {
	// if mir.isCurrentPrimary(from) {
	// 	mir.logger.Errorf("Prepare received vcSender Primary %v: %v", from, m)
	// 	return
	// }

	inst, exists := mir.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, mir)
		mir.log[m.Index] = inst
	} else if !mir.updateCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.InstanceState_Prepared
		cm := &pb.AgreementMessage{
			Epoch:       m.Epoch,
			Index:       m.Index,
			Type:        pb.AgreementMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}
		mir.broadcast(cm, false)

		inst.cCert.log(mir.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.InstanceState_Committed
			mir.exec()
		}
	}
}

// HandleP2b handles P2b message
func (mir *MirBFT) onCommit(m *pb.AgreementMessage, from peerpb.PeerID) {
	inst, exists := mir.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, mir)
		mir.log[m.Index] = inst
	} else if !mir.updateCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m)
	if inst.cCert.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.InstanceState_Committed
		mir.exec()
	}
	//p.logger.Debugf("commit acks for %v: %v", m.Command.ID, len(p.log[m.Index].cCert.msgs))
}

func (mir *MirBFT) exec() {
	for {
		inst, ok := mir.log[mir.nextDeliverIndex]
		if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
			mir.logger.Debugf("instance %v not committed yet: %v", mir.nextDeliverIndex, inst)
			break
		}

		mir.logger.Debugf("Replica %d execute [s=%d, cmd=%d]\n", mir.id, mir.nextDeliverIndex,
			inst.is.Command.Timestamp)

		mir.enqueueForExecution(inst.is.Command)
		inst.is.Command = nil
		inst.is.Status = pb.InstanceState_Executed
		mir.nextDeliverIndex++
	}
}

func (mir *MirBFT) enqueueForExecution(cmd *commandpb.Command) {
	mir.toDeliver = append(mir.toDeliver, peer.ExecPacket{Cmd: *cmd})
}

func (mir *MirBFT) leaderOfSeq(e pb.Epoch, seq pb.Index) peerpb.PeerID {
	return peerpb.PeerID(seq) % peerpb.PeerID(len(mir.epochLeaders[e]))
}
