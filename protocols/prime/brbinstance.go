package prime

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

type instance struct {
	p *prime

	is pb.InstanceState

	pCert *quorum
	cCert *quorum
}

func makeInstance(m *pb.PreOrderMessage, p *prime) *instance {
	return &instance{
		is: pb.InstanceState{
			InstanceID:  m.InstanceID,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		pCert: newQuorum(p),
		cCert: newQuorum(p),
	}
}

func (p *prime) updateCommand(inst *instance, m *pb.PreOrderMessage) bool {
	if inst.is.Command != nil {
		if !inst.is.Command.Equal(m.Command) {
			p.logger.Errorf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Command, m.Command)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.Command = m.Command
	}
	return true
}

func (p *prime) updateCommandHash(inst *instance, m *pb.PreOrderMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			p.logger.Errorf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.is.CommandHash, m.CommandHash)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (p *prime) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	cmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	compHash := sha256.Sum256(cmdBytes)
	return bytes.Equal(hash, compHash[:])
}

func (p *prime) onPORequest(m *pb.PreOrderMessage, from peerpb.PeerID) {
	instID := m.InstanceID
	inst, exists := p.log[instID]

	if !p.checkCommandHash(m.Command, m.CommandHash) {
		p.logger.Panicf("The hash does not correspond to the command: %v", m)
	}

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.InstanceID] = inst
	} else if !p.updateCommand(inst, m) || !p.updateCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	m.Command = nil
	inst.pCert.log(from, m)
	inst.is.Status = pb.InstanceState_PreOrder
	pm := &pb.PreOrderMessage{
		InstanceID:  inst.is.InstanceID,
		Type:        pb.PreOrderMessage_POAck,
		CommandHash: inst.is.CommandHash,
	}
	inst.pCert.log(p.id, pm)
	p.broadcast(pm, false)
	if inst.pCert.Majority(pm) && inst.is.IsPreOrdered() {
		inst.is.Status = pb.InstanceState_PreOrderAck
		p.summarize(instID)
	}
}

func (p *prime) onPOAck(m *pb.PreOrderMessage, from peerpb.PeerID) {
	instID := m.InstanceID
	inst, exists := p.log[instID]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[instID] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPreOrdered() {
		inst.is.Status = pb.InstanceState_PreOrderAck
		p.summarize(instID)
	}
}

func (p *prime) summarize(instID pb.InstanceID) {
	i := p.poSummary[instID.ReplicaID] + 1
	for {
		inst, ok := p.log[pb.InstanceID{ReplicaID: instID.ReplicaID, Index: i}]
		if !ok || !inst.is.IsPOAcked() || inst.is.Command == nil {
			break
		}
		i++
	}
	p.poSummary[instID.ReplicaID] = i - 1
}

func (p *prime) sendSummary() {
	sMsg := &pb.PreOrderMessage{
		Type: pb.PreOrderMessage_POSummary,
		POSummary: pb.POSummary{
			POSummary: p.poSummary,
		},
	}
	p.lastPreorderSummaries.POSummaryMatrix[p.id] = sMsg.POSummary

	summaryBytes, err := proto.Marshal(&p.lastPreorderSummaries)
	if err != nil {
		panic(err)
	}
	summaryHash := sha256.Sum256(summaryBytes)

	p.lastPreorderSummariesHash = summaryHash[:]

	p.broadcast(sMsg, false)
}

func (p *prime) onPOSummary(m *pb.PreOrderMessage, from peerpb.PeerID) {
	p.logger.Debugf("Received POSummary from %v: %v", from, m.POSummary)
	p.lastPreorderSummaries.POSummaryMatrix[from] = m.POSummary

	summaryBytes, err := proto.Marshal(&p.lastPreorderSummaries)
	if err != nil {
		panic(err)
	}
	summaryHash := sha256.Sum256(summaryBytes)

	p.lastPreorderSummariesHash = summaryHash[:]
}
