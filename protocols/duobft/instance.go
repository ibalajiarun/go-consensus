package duobft

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
)

type instance struct {
	p *duobft

	is pb.InstanceState

	tcCert *Quorum
	pcCert *Quorum
	cCert  *Quorum
}

func makeInstance(m *pb.NormalMessage, p *duobft) *instance {
	return &instance{
		is: pb.InstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.InstanceState_None,
			TStatus:     pb.InstanceState_TNone,
			Command:     m.Command,
			CommandHash: m.CommandHash,
		},
		tcCert: newQuorum(p, p.f+1),
		pcCert: newQuorum(p, 2*p.f+1),
		cCert:  newQuorum(p, 2*p.f+1),
	}
}

func (s *duobft) updateCommand(inst *instance, m *pb.NormalMessage) bool {
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

func (s *duobft) updateCommandHash(inst *instance, m *pb.NormalMessage) bool {
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

func (s *duobft) checkCommandHash(cmd *commandpb.Command, hash []byte) bool {
	cmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	cmpHash := sha256.Sum256(cmdBytes)
	return bytes.Equal(hash, cmpHash[:])
}

func (p *duobft) onPrepare(m *pb.NormalMessage, from peerpb.PeerID, mSign []byte) {
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

	inst.tcCert.log(from, m)
	inst.pcCert.log(from, m)

	inst.is.Status = pb.InstanceState_Prepared
	inst.is.TStatus = pb.InstanceState_TPrepared
	pcm := &pb.NormalMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.NormalMessage_PreCommit,
		CommandHash: inst.is.CommandHash,
	}

	mBytes := p.marshall(pcm)

	p.dispatcher2.Exec(
		func() []byte {
			sign := p.usigSign(mBytes)
			return sign
		},
		func(sign []byte) {
			p.broadcast(mBytes, false, sign)

			inst.tcCert.log(p.id, pcm)
			inst.pcCert.log(p.id, pcm)

			if inst.is.IsTPrepared() && inst.tcCert.Majority(pcm) {
				inst.is.TStatus = pb.InstanceState_TCommitted
				p.texec()
			}

			if inst.is.IsPrepared() && inst.pcCert.Majority(pcm) {
				inst.is.Status = pb.InstanceState_PreCommitted
				cm := &pb.NormalMessage{
					View:        pcm.View,
					Index:       pcm.Index,
					Type:        pb.NormalMessage_Commit,
					CommandHash: inst.is.CommandHash,
				}

				mBytes := p.marshall(cm)

				p.dispatcher.Exec(
					func() []byte {
						sign := p.normalSign(mBytes)
						return sign
					},
					func(sign []byte) {
						p.broadcast(mBytes, false, sign)

						inst.cCert.log(p.id, cm)
						if inst.cCert.Majority(cm) && inst.is.IsPreCommitted() {
							inst.is.Status = pb.InstanceState_Committed
							p.exec()
						}
					},
				)
			}
		},
	)
}

func (p *duobft) onPreCommit(m *pb.NormalMessage, from peerpb.PeerID, mSign []byte) {
	inst, exists := p.log[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeInstance(m, p)
		p.log[m.Index] = inst
	} else if !p.updateCommandHash(inst, m) {
		return
	}

	inst.tcCert.log(from, m)
	inst.pcCert.log(from, m)

	if inst.is.IsTPrepared() && inst.tcCert.Majority(m) {
		inst.is.TStatus = pb.InstanceState_TCommitted
		p.texec()
	}

	if inst.is.IsPrepared() && inst.pcCert.Majority(m) {
		inst.is.Status = pb.InstanceState_PreCommitted
		cm := &pb.NormalMessage{
			View:        m.View,
			Index:       m.Index,
			Type:        pb.NormalMessage_Commit,
			CommandHash: inst.is.CommandHash,
		}

		mBytes := p.marshall(cm)

		p.dispatcher.Exec(
			func() []byte {
				sign := p.normalSign(mBytes)
				return sign
			}, func(sign []byte) {
				p.broadcast(mBytes, false, sign)

				inst.cCert.log(p.id, cm)
				if inst.is.IsPreCommitted() && inst.cCert.Majority(cm) {
					inst.is.Status = pb.InstanceState_Committed
					p.exec()
				}
			},
		)
	}
}

func (p *duobft) onCommit(m *pb.NormalMessage, from peerpb.PeerID) {
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
	if inst.is.IsPreCommitted() && inst.cCert.Majority(m) {
		inst.is.Status = pb.InstanceState_Committed
		p.exec()
	}
}

func (p *duobft) texec() {
	for {
		inst, ok := p.log[p.tExecute]
		if !ok || !inst.is.IsTCommitted() || inst.is.Command == nil {
			break
		}

		p.logger.Debugf("Replica %d texecute [s=%d, cmd=%d]\n", p.id, p.tExecute,
			inst.is.Command)

		cmd := inst.is.Command
		if cmd.Meta != nil {
			p.Execute(cmd)
		}
		inst.is.TStatus = pb.InstanceState_TExecuted
		p.tExecute++
	}
}

func (p *duobft) exec() {
	for {
		inst, ok := p.log[p.execute]
		if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
			break
		}

		p.logger.Debugf("Replica %d execute [s=%d, cmd=%d]\n", p.id, p.execute,
			inst.is.Command)

		cmd := inst.is.Command
		if cmd.Meta == nil {
			p.Execute(cmd)
		}

		inst.is.Status = pb.InstanceState_Executed
		p.execute++
	}
}

func (p *duobft) isCurrentPrimary(id peerpb.PeerID) bool {
	return p.isPrimaryAtView(id, p.curView)
}

func (p *duobft) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(p.nodes))
}
