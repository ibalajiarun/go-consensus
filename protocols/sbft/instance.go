package sbft

import (
	"bytes"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"
)

type instance struct {
	s *sbft
	// traceInfo opentracing.TextMapCarrier
	is    pb.InstanceState
	cCert *quorum
}

func makeInstance(m *pb.NormalMessage, s *sbft) *instance {
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

func (s *sbft) updateCommand(inst *instance, m *pb.NormalMessage) bool {
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

func (s *sbft) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
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

func (s *sbft) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	compHash := cmd.Hash()
	return bytes.Equal(hash, compHash[:])
}

func (s *sbft) onPrepare(m *pb.NormalMessage, from peerpb.PeerID, mSign []byte) {
	inst, exists := s.log[m.Index]

	// span, ctx := opentracing.StartSpanFromContext(ctx, "onPrepare")
	// defer span.Finish()

	cmdHash := m.Command.Hash()
	m.CommandHash = cmdHash

	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, s)
		s.log[m.Index] = inst
	} else if !s.updateCommand(inst, m) || !s.updateCommandHash(inst, m) {
		return
	}

	if inst.is.Status == pb.InstanceState_Committed {
		s.exec()
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
		// span := opentracing.StartSpan("onPrepare:Sign", opentracing.ChildOf(span.Context()))
		// defer span.Finish()
		return s.signer.Sign(mBytes, 0)
	}, func(ssmSign []byte) {
		s.sendTo(from, mBytes, ssmSign)
	})
}

func (s *sbft) onSignShare(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	// span, ctx := opentracing.StartSpanFromContext(ctx, "onSignShare")
	// defer span.Finish()

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

			// span.SetTag("majority", true)
			// span.SetTag("from", from)

			inst.is.Status = pb.InstanceState_SignShared
			cm := &pb.NormalMessage{
				View:        m.View,
				Index:       m.Index,
				Type:        pb.NormalMessage_Commit,
				CommandHash: m.CommandHash,
			}

			mBytes := s.marshall(cm)

			// buf := make(opentracing.TextMapCarrier)
			// if err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, opentracing.TextMapWriter(buf)); err != nil {
			// 	s.logger.Panicf("error injecting trace: %w", err)
			// }
			// inst.traceInfo = buf

			s.aggDispatcher.Exec(func() []byte {
				// span := opentracing.StartSpan("onSignShare:Aggregate", opentracing.ChildOf(span.Context()))
				// defer span.Finish()
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

func (s *sbft) onCommit(m *pb.NormalMessage, from peerpb.PeerID, sign []byte, content []byte) {
	// span, ctx := opentracing.StartSpanFromContext(ctx, "onCommit")
	// defer span.Finish()

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
		s.exec()
		return
	}

	pType := m.Type
	m.Type = pb.NormalMessage_SignShare
	mBytes := s.marshall(m)
	m.Type = pType

	// buf := make(opentracing.TextMapCarrier)
	// if err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, opentracing.TextMapWriter(buf)); err != nil {
	// 	s.logger.Panicf("error injecting trace: %w", err)
	// }
	// inst.traceInfo = buf

	s.verifyDispatcher.Exec(func() []byte {
		// span := opentracing.StartSpan("onCommit:Verify", opentracing.ChildOf(span.Context()))
		// defer span.Finish()
		s.signer.Verify(mBytes, 0, sign)
		return nil
	}, func(b []byte) {
		inst.is.Status = pb.InstanceState_Committed
		s.exec()
	})
}

func (s *sbft) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(s.nodes))
}
