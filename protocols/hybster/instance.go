package hybster

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybster/hybsterpb"
)

type prPkg struct {
	pr *pb.NormalMessage
	// hash    [32]byte
	content []byte
	cert    []byte
	from    peerpb.PeerID
}

type vcPkg struct {
	vc *pb.ViewChangeMessage
	// hash    [32]byte
	content []byte
	cert    []byte
	from    peerpb.PeerID
}

type nvPkg struct {
	nv *pb.NewViewMessage
	// hash    [32]byte
	content []byte
	cert    []byte
}

type nvaPkg struct {
	nva     *pb.NewViewAckMessage
	content []byte
	cert    []byte
	from    peerpb.PeerID
}

type instance struct {
	p *hybster

	slot        pb.Order
	view        pb.View
	command     *commandpb.Command
	commandHash []byte

	prepared bool
	commit   bool
	executed bool
	quorum   *Quorum
}

func makeInstance(m *pb.NormalMessage, p *hybster) *instance {
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

func (inst *instance) reset() {
	inst.quorum.Reset()
	inst.prepared = false
	inst.commit = false
	inst.executed = false
}

func (s *hybster) updateCommand(inst *instance, m *pb.NormalMessage) bool {
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

func (s *hybster) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
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

func (p *hybster) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	compHash := cmd.Hash()
	return bytes.Equal(hash, compHash[:])
}

func (p *hybster) onPrepare(m *pb.NormalMessage, from peerpb.PeerID) {
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

	// update instance
	inst, exists := p.log[m.Order]

	if !p.checkCommandHash(m.Command, m.CommandHash) {
		p.logger.Panicf("The hash does not correspond to the command: %v", m)
	}

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Order] = inst
	} else if !p.updateCommand(inst, m) || !p.updateCommandHash(inst, m) {
		return
	}

	m.Command = nil
	inst.quorum.log(from, m)
	inst.prepared = true

	cm := &pb.NormalMessage{
		View:        m.View,
		Order:       m.Order,
		Type:        pb.NormalMessage_Commit,
		CommandHash: m.CommandHash,
	}
	p.broadcastNormal(cm)
	inst.quorum.log(p.id, cm)

	if inst.prepared && !inst.commit && inst.quorum.Majority(m) {
		inst.commit = true
		p.exec()
	}
}

// HandleP2b handles P2b message
func (p *hybster) onCommit(m *pb.NormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %v ===[%v]===>>> Replica %v\n", from, m,
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
}

func (p *hybster) exec() {
	for {
		e, ok := p.log[p.execute]
		if !ok || !e.commit || e.command == nil {
			break
		}

		p.logger.Debugf("Replica %s execute [s=%d, cmd=%v]\n", p.id, p.execute,
			e.command)

		p.Execute(e.command)
		e.command = nil
		e.executed = true
		p.execute++
	}
}
