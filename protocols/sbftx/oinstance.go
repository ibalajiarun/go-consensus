package sbftx

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/protocols/sbftx/sbftxpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbftx/sbftxpb"
)

type oinstance struct {
	s     *sbftx
	is    pb.OInstanceState
	cCert *oquorum
}

func makeOInstance(m *pb.ONormalMessage, s *sbftx) *oinstance {
	return &oinstance{
		is: pb.OInstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.OInstanceState_None,
			Instances:   m.Instances,
			CommandHash: m.CommandHash,
		},
		cCert: newOQuorum(s),
	}
}

func (s *sbftx) updateOCommand(inst *oinstance, m *pb.ONormalMessage) bool {
	if inst.is.Instances != nil {
		if !sbftxpb.InstancesEquals(inst.is.Instances, m.Instances) {
			s.logger.Panicf("Different ocommand for same instance %v: Has: %v, Received %v.", inst.is, inst.is.Instances, m.Instances)
			return false
		}
	} else {
		inst.is.Instances = m.Instances
	}
	return true
}

func (s *sbftx) updateOCommandHash(inst *oinstance, m *pb.ONormalMessage) bool {
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

func (s *sbftx) onCRequest(id pb.InstanceID) {
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

func (s *sbftx) sendCRequest() {
	if !s.isPrimaryAtView(s.id, s.oview) {
		s.logger.Errorf("Not the leader of view %d: %d", s.oview, s.id)
		return
	}

	ids := s.cReqBuffer
	s.cReqBuffer = nil

	if len(ids) <= 0 {
		return
	}

	s.logger.Debugf("sendCRequest: %v", ids)
	index := s.oindex.GetAndIncrement()
	inst := &oinstance{
		s: s,
		is: pb.OInstanceState{
			View:      s.view,
			Index:     index,
			Status:    pb.OInstanceState_Prepared,
			Instances: ids,
		},
		cCert: newOQuorum(s),
	}
	s.olog[index] = inst

	pm := &pb.ONormalMessage{
		View:      s.view,
		Index:     index,
		Type:      pb.ONormalMessage_OPrepare,
		Instances: ids,
	}

	mBytes := s.marshall(pm)

	s.broadcast(mBytes, true /* sendToSelf */, nil)
}

func (s *sbftx) onOPrepare(m *pb.ONormalMessage, from peerpb.PeerID, mSign []byte) {
	inst, exists := s.olog[m.Index]

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
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommand(inst, m) || !s.updateOCommandHash(inst, m) {
		s.logger.Panicf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Instances, m.Instances)
		return
	}

	if inst.is.Status == pb.OInstanceState_Committed {
		s.exec()
		return
	}

	inst.is.Status = pb.OInstanceState_Prepared
	ssm := &pb.ONormalMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.ONormalMessage_OSignShare,
		CommandHash: inst.is.CommandHash,
	}

	mBytes := s.marshall(ssm)
	s.signDispatcher.Exec(func() []byte {
		return s.signer.Sign(mBytes, 0)
	}, func(ssmSign []byte) {
		s.sendTo(from, mBytes, ssmSign)
	})
}

func (s *sbftx) onOSignShare(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		s.logger.Errorf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Instances, m.Instances)
		return
	}

	inst.cCert.log(from, m, sign)

	if inst.is.Status < pb.OInstanceState_SignShared {
		if majSigs, majority := inst.cCert.Majority(m); majority {
			inst.is.Status = pb.OInstanceState_SignShared
			cm := &pb.ONormalMessage{
				View:        m.View,
				Index:       m.Index,
				Type:        pb.ONormalMessage_OCommit,
				CommandHash: m.CommandHash,
			}

			mBytes := s.marshall(cm)

			s.aggDispatcher.Exec(func() []byte {
				return s.signer.AggregateSigAndVerify(content, 0, majSigs)
			}, func(newSig []byte) {
				s.broadcast(mBytes, false, newSig)
				if true {
					inst.is.Status = pb.OInstanceState_Committed
					s.exec()
				}
			})
		}
	}
}

func (s *sbftx) onOCommit(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		s.logger.Errorf("Different commands for same instance %v: Received: %v, Has %v.", inst, inst.is.Instances, m.Instances)
		return
	}

	if inst.is.Status >= pb.OInstanceState_Committed {
		s.exec()
		return
	}

	pType := m.Type
	m.Type = pb.ONormalMessage_OSignShare
	mBytes := s.marshall(m)
	m.Type = pType

	s.verifyDispatcher.Exec(func() []byte {
		s.signer.Verify(mBytes, 0, sign)
		return nil
	}, func(b []byte) {
		inst.is.Status = pb.OInstanceState_Committed
		s.exec()
	})
}
