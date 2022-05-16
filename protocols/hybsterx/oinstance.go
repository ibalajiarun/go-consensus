package hybsterx

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

type oinstance struct {
	p      *hybsterx
	is     pb.OInstanceState
	quorum *OQuorum
}

func makeOInstance(m *pb.ONormalMessage, p *hybsterx) *oinstance {
	return &oinstance{
		p: p,
		is: pb.OInstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.OInstanceState_None,
			Instances:   m.Instances,
			CommandHash: m.CommandHash,
		},
		quorum: newOQuorum(p),
	}
}

func (s *hybsterx) updateOCommand(inst *oinstance, m *pb.ONormalMessage) bool {
	if inst.is.Instances != nil {
		if !hybsterxpb.InstancesEquals(inst.is.Instances, m.Instances) {
			s.logger.Panicf("Different ocommand for same instance %v: Has: %v, Received %v.", inst.is, inst.is.Instances, m.Instances)
			return false
		}
	} else {
		inst.is.Instances = m.Instances
	}
	return true
}

func (s *hybsterx) updateOCommandHash(inst *oinstance, m *pb.ONormalMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			s.logger.Panicf("Different ocommand hashes for same instance %v: Has: %v, Received %v.", inst.is, inst.is.CommandHash, m.CommandHash)
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (s *hybsterx) onCRequest(id pb.InstanceID) {
	s.logger.Debugf("Replica %v received CRequest(%v)\n", s.id, id)
	if !s.isPrimaryAtView(s.id, s.oview) {
		s.logger.Errorf("Not the leader of view %d: %d", s.oview, s.id)
		return
	}

	s.cReqBuffer = append(s.cReqBuffer, id)
	if len(s.cReqBuffer) >= int(s.oBatchSize) {
		s.sendCRequest()
	}
}

func (p *hybsterx) sendCRequest() {
	if !p.isPrimaryAtView(p.id, p.oview) {
		p.logger.Errorf("Not the leader of view %d: %d", p.oview, p.id)
		return
	}

	ids := p.cReqBuffer
	p.cReqBuffer = nil

	if len(ids) <= 0 {
		return
	}

	hasher := sha256.New()
	for _, instID := range ids {
		bytes, err := proto.Marshal(&instID)
		if err != nil {
			panic(err)
		}
		hasher.Write(bytes)
	}
	cmdHash := hasher.Sum(nil)

	p.logger.Debugf("sendCRequest: %v", ids)
	index := p.oindex.GetAndIncrement()

	inst := &oinstance{
		p: p,
		is: pb.OInstanceState{
			View:        p.view,
			Index:       index,
			Status:      pb.OInstanceState_Prepared,
			Instances:   ids,
			CommandHash: cmdHash,
		},
		quorum: newOQuorum(p),
	}
	p.olog[index] = inst

	pm := &pb.ONormalMessage{
		View:        p.view,
		Index:       index,
		Type:        pb.ONormalMessage_OPrepare,
		Instances:   ids,
		CommandHash: cmdHash,
	}
	inst.quorum.log(p.id, pm)

	p.broadcastONormal(pm)
}

func (p *hybsterx) onOPrepare(m *pb.ONormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %d ===[OPrepare %v]===>>> Replica %d\n", from, m,
		p.id)

	// update instance
	inst, exists := p.olog[m.Index]

	hasher := sha256.New()
	for _, instID := range m.Instances {
		bytes, err := proto.Marshal(&instID)
		if err != nil {
			panic(err)
		}
		hasher.Write(bytes)
	}
	cmdHash := hasher.Sum(nil)
	m.CommandHash = cmdHash

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, p)
		p.olog[m.Index] = inst
	} else if !p.updateOCommand(inst, m) || !p.updateOCommandHash(inst, m) {
		return
	}

	inst.quorum.log(from, m)
	inst.is.Status = pb.OInstanceState_Prepared

	cm := &pb.ONormalMessage{
		View:        m.View,
		Index:       m.Index,
		Type:        pb.ONormalMessage_OCommit,
		CommandHash: m.CommandHash,
	}
	p.broadcastONormal(cm)
	inst.quorum.log(p.id, cm)

	if inst.quorum.Majority(cm) && inst.is.IsPrepared() {
		inst.is.Status = pb.OInstanceState_Committed
		p.exec()
	}
}

// HandleP2b handles P2b message
func (p *hybsterx) onOCommit(m *pb.ONormalMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %s ===[%v]===>>> Replica %s\n", from, m,
		p.id)

	inst, exists := p.olog[m.Index]

	// update instance
	if !exists {
		inst = makeOInstance(m, p)
		p.olog[m.Index] = inst
	} else if !p.updateOCommandHash(inst, m) {
		return
	}

	inst.quorum.log(from, m)
	if inst.quorum.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.OInstanceState_Committed
		p.exec()
	}
}
