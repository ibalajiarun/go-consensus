package linhybster

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/linhybster/linhybsterpb"
)

type instance struct {
	s     *linhybster
	is    pb.InstanceState
	cCert *quorum
}

func makeInstance(m *pb.NormalMessage, s *linhybster) *instance {
	return &instance{
		is: pb.InstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.InstanceState_None,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		cCert: newQuorum(s),
	}
}

func (s *linhybster) updateCommand(inst *instance, m *pb.NormalMessage) bool {
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

func (s *linhybster) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			s.logger.Panicf("Different command hashes for same instance %v: Has: %v, Received %v.", inst, inst.is.CommandHash, m.CommandHash)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (s *linhybster) onPrepare(m *pb.NormalMessage, from peerpb.PeerID, mSign []byte) {
	inst, exists := s.log[m.Index]

	m.CommandHash = m.Command.Hash()

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, s)
		s.log[m.Index] = inst
	} else if !s.updateCommand(inst, m) || !s.updateCommandHash(inst, m) {
		return
	}

	inst.is.Status = pb.InstanceState_Prepared
	ssm := &pb.NormalMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.NormalMessage_SignShare,
		CommandHash: inst.is.CommandHash,
	}

	mBytes := s.marshall(ssm)

	s.signDispatcher.Exec(func() []byte {
		return s.signer.SignUsingEnclave(mBytes, 0)
	}, func(ssmSign []byte) {
		s.sendTo(from, mBytes, ssmSign)
	})
}

func (s *linhybster) onSignShare(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, s)
		s.log[m.Index] = inst
	} else if !s.updateCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m, sign)

	if inst.is.Status < pb.InstanceState_SignShared {
		if majSigs, majority := inst.cCert.Majority(m); majority {
			inst.is.Status = pb.InstanceState_SignShared
			cm := &pb.NormalMessage{
				View:        m.View,
				Index:       m.Index,
				Type:        pb.NormalMessage_Commit,
				CommandHash: m.CommandHash,
			}
			mBytes := s.marshall(cm)

			s.aggDispatcher.Exec(func() []byte {
				return s.signer.AggregateSigAndVerify(content, 0, majSigs)
			}, func(newSig []byte) {
				s.broadcast(mBytes, false, newSig)
				if true {
					inst.is.Status = pb.InstanceState_Committed
					s.exec()
				}
			})
		}
	}
}

func (s *linhybster) onCommit(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	inst, exists := s.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, s)
		s.log[m.Index] = inst
	} else if !s.updateCommandHash(inst, m) {
		return
	}

	if inst.is.Status >= pb.InstanceState_Committed {
		return
	}

	// s.logger.Debugf("OnCommit len=%v sig=%v", len(content), sign)

	pType := m.Type
	m.Type = pb.NormalMessage_SignShare
	mBytes := s.marshall(m)
	m.Type = pType

	s.verifyDispatcher.Exec(func() []byte {
		s.signer.Verify(mBytes, 0, sign)
		return nil
	}, func(b []byte) {
		inst.is.Status = pb.InstanceState_Committed
		s.exec()
	})
}

func (s *linhybster) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(s.nodes))
}
