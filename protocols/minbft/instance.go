package minbft

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/minbft/minbftpb"
)

type prPkg struct {
	pr      pb.NormalMessage
	hash    []byte
	content []byte
	cert    []byte
	from    peerpb.PeerID
}

type instance struct {
	p *minbft

	slot        pb.Order
	view        pb.View
	command     *commandpb.Command
	commandHash []byte

	prepared bool
	commit   bool
	executed bool
	quorum   *Quorum
}

func makeInstance(m *pb.NormalMessage, p *minbft) *instance {
	return &instance{
		p:           p,
		view:        m.View,
		slot:        m.Order,
		command:     m.Command,
		commandHash: m.CommandHash,
		commit:      false,
		quorum:      newQuorum(p),
	}
}

func (s *minbft) updateCommand(inst *instance, m *pb.NormalMessage) bool {
	if inst.command != nil {
		if !inst.command.Equal(m.Command) {
			s.logger.Panicf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.command = m.Command
	}
	return true
}

func (s *minbft) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
	if inst.commandHash != nil {
		if !bytes.Equal(inst.commandHash, m.CommandHash) {
			s.logger.Panicf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.commandHash = m.CommandHash
	}
	return true
}

func (p *minbft) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	compHash := cmd.Hash()
	return bytes.Equal(hash, compHash[:])
}

func (p *minbft) onPrepare(m *pb.NormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %d ===[Prepare %v]===>>> Replica %d\n", from, m,
		p.id)

	if m.View < p.curView {
		p.logger.Debugf("I have moved on to another view %v. %v", p.curView, m)
		return
	}

	// update slot number
	if p.slot < m.Order {
		p.slot = m.Order
	}

	if !p.checkCommandHash(m.Command, m.CommandHash) {
		p.logger.Panicf("The hash does not correspond to the command: %v", m)
	}

	inst, exists := p.log[m.Order]

	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Order] = inst
	} else if !p.updateCommand(inst, m) || !p.updateCommandHash(inst, m) {
		return
	}

	inst.quorum.log(from, m)
	inst.prepared = true

	cm := &pb.NormalMessage{
		View:        m.View,
		Order:       m.Order,
		Type:        pb.NormalMessage_Commit,
		CommandHash: m.CommandHash,
	}

	inst.quorum.log(p.id, cm)
	p.broadcastNormal(cm)

	if inst.prepared && !inst.commit && inst.quorum.Majority(m) {
		inst.commit = true
		p.exec()
	}
}

// HandleP2b handles P2b message
func (p *minbft) onCommit(m *pb.NormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %s ===[%v]===>>> Replica %s\n", from, m,
		p.id)

	// update slot number
	if p.slot < m.Order {
		p.slot = m.Order
	}

	inst, exists := p.log[m.Order]

	// update instance
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Order] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.quorum.log(from, m)
	if inst.prepared && !inst.commit && inst.quorum.Majority(m) {
		inst.commit = true
		p.exec()
	}

	// p.logger.Debugf("ACKS %v", p.log[m.Order].quorum.acks)
}

func (p *minbft) exec() {
	for {
		e, ok := p.log[p.execute]
		if !ok || !e.commit {
			break
		}

		p.logger.Debugf("Replica %s execute [s=%d, cmd=%v]\n", p.id, p.execute,
			e.command)

		p.Execute(e.command)
		e.executed = true
		p.execute++
	}
}
