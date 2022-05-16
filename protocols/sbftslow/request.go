package sbftslow

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbftslow/sbftslowpb"
)

func (s *sbftslow) onRequest(cmd *commandpb.Command) *instance {
	s.logger.Debugf("Replica %v received command ID %v\n", s.id, cmd.Timestamp)

	index := s.index.GetAndIncrement()
	inst := &instance{
		s: s,
		is: pb.InstanceState{
			View:    s.view,
			Index:   index,
			Status:  pb.InstanceState_Preprepared,
			Command: cmd,
		},
		fcCert: newQuorum(s),
		scCert: newQuorum(s),
	}
	s.log[index] = inst
	s.logger.Debugf("onRequest inst %v", index)

	pm := &pb.NormalMessage{
		View:    s.view,
		Index:   index,
		Type:    pb.NormalMessage_Preprepare,
		Command: cmd,
	}
	mBytes := s.marshall(pm)

	s.broadcast(mBytes, true /* sendToSelf */, nil)

	return inst
}
