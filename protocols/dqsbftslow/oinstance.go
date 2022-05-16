package dqsbftslow

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/protocols/dqsbftslow/dqsbftslowpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqsbftslow/dqsbftslowpb"
)

type oinstance struct {
	s      *dqsbftslow
	is     pb.OInstanceState
	fcCert *oquorum
	scCert *oquorum
}

func makeOInstance(m *pb.ONormalMessage, s *dqsbftslow) *oinstance {
	return &oinstance{
		is: pb.OInstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.OInstanceState_None,
			Instances:   m.Instances,
			CommandHash: m.CommandHash,
		},
		fcCert: newOQuorum(s),
		scCert: newOQuorum(s),
	}
}

func (s *dqsbftslow) updateOCommand(inst *oinstance, m *pb.ONormalMessage) bool {
	if inst.is.Instances != nil {
		if !dqsbftslowpb.InstancesEquals(inst.is.Instances, m.Instances) {
			s.logger.Panicf("Different ocommand for same instance %v: Has: %v, Received %v.", inst.is, inst.is.Instances, m.Instances)
			return false
		}
	} else {
		inst.is.Instances = m.Instances
	}
	return true
}

func (s *dqsbftslow) updateOCommandHash(inst *oinstance, m *pb.ONormalMessage) bool {
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

func (s *dqsbftslow) onCRequest(id pb.InstanceID) {
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

func (s *dqsbftslow) sendCRequest() {
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
			Status:    pb.OInstanceState_Preprepared,
			Instances: ids,
		},
		fcCert: newOQuorum(s),
		scCert: newOQuorum(s),
	}
	s.olog[index] = inst

	pm := &pb.ONormalMessage{
		View:      s.view,
		Index:     index,
		Type:      pb.ONormalMessage_Preprepare,
		Instances: ids,
	}

	mBytes := s.marshall(pm)

	s.broadcast(mBytes, true /* sendToSelf */, nil)
}

func (s *dqsbftslow) onOPreprepare(m *pb.ONormalMessage, from peerpb.PeerID, mSign []byte) {
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
		return
	}

	if inst.is.Status == pb.OInstanceState_Committed {
		s.exec()
		return
	}

	if inst.is.Status > pb.OInstanceState_Preprepared {
		return
	}

	inst.is.Status = pb.OInstanceState_Preprepared
	ssm := &pb.ONormalMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.ONormalMessage_SignShare,
		CommandHash: inst.is.CommandHash,
	}

	mBytes := s.marshall(ssm)
	s.signDispatcher.Exec(func() []byte {
		return s.signer.Sign(mBytes, 0)
	}, func(ssmSign []byte) {
		s.sendTo(from, mBytes, ssmSign)
	})
}

func (s *dqsbftslow) onOSignShare(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		return
	}

	inst.fcCert.log(from, m, sign)

	if inst.is.Status < pb.OInstanceState_SignShared {
		if majSigs, majority := inst.fcCert.Majority(m); majority {
			inst.is.Status = pb.OInstanceState_SignShared
			pm := &pb.ONormalMessage{
				View:        m.View,
				Index:       m.Index,
				Type:        pb.ONormalMessage_Prepare,
				CommandHash: m.CommandHash,
			}

			mBytes := s.marshall(pm)
			s.aggDispatcher.Exec(func() []byte {
				return s.signer.AggregateSigAndVerify(content, 0, majSigs)
			}, func(newSig []byte) {
				s.broadcast(mBytes, true, newSig)
			})
		}
	}
}

func (s *dqsbftslow) onOPrepare(m *pb.ONormalMessage, from peerpb.PeerID, mSign []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		return
	}

	if inst.is.Status > pb.OInstanceState_Prepared {
		return
	}

	pType := m.Type
	m.Type = pb.ONormalMessage_SignShare
	mBytes := s.marshall(m)
	m.Type = pType

	inst.is.Status = pb.OInstanceState_Prepared
	ssm := &pb.ONormalMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.ONormalMessage_CommitSig,
		CommandHash: inst.is.CommandHash,
	}

	s.verifyDispatcher.Exec(func() []byte {
		s.signer.Verify(mBytes, 0, mSign)
		return nil
	}, func(b []byte) {
		ssmBytes := s.marshall(ssm)
		s.signDispatcher.Exec(func() []byte {
			return s.signer.Sign(ssmBytes, 0)
		}, func(ssmSign []byte) {
			s.sendTo(from, ssmBytes, ssmSign)
		})
	})

}

func (s *dqsbftslow) onOCommitSig(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		return
	}
	inst.scCert.log(from, m, sign)
	if inst.is.Status < pb.OInstanceState_CommitSigShared {
		if majSigs, majority := inst.scCert.Majority(m); majority {
			inst.is.Status = pb.OInstanceState_CommitSigShared
			cm := &pb.ONormalMessage{
				View:        m.View,
				Index:       m.Index,
				Type:        pb.ONormalMessage_CommitSlow,
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

func (s *dqsbftslow) onOCommit(m *pb.ONormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, s)
		s.olog[m.Index] = inst
	} else if !s.updateOCommandHash(inst, m) {
		return
	}

	if inst.is.Status >= pb.OInstanceState_Committed {
		return
	}

	pType := m.Type
	m.Type = pb.ONormalMessage_CommitSig
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
