package linhybster

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/linhybster/linhybsterpb"
)

func (s *linhybster) onRequest(cmd *commandpb.Command) *instance {
	if !s.isPrimaryAtView(s.id, s.view) {
		s.logger.Panicf("Fix your client config. I am not the leader: %v", s.id)
	}
	s.logger.Debugf("Replica %v received command ID %v\n", s.id, cmd.Timestamp)

	index := s.index.GetAndIncrement()
	inst := &instance{
		s: s,
		is: pb.InstanceState{
			View:    s.view,
			Index:   index,
			Status:  pb.InstanceState_Prepared,
			Command: cmd,
		},
		cCert: newQuorum(s),
	}
	s.log[index] = inst
	s.logger.Debugf("onRequest inst %v", index)

	pm := &pb.NormalMessage{
		View:    s.view,
		Index:   index,
		Type:    pb.NormalMessage_Prepare,
		Command: cmd,
	}

	mBytes := s.marshall(pm)

	s.broadcast(mBytes, true /* sendToSelf */, nil)

	return inst
}
