package destiny

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/destiny/destinypb"
)

func (s *destiny) onRequest(cmd *commandpb.Command) *instance {
	s.logger.Debugf("Replica %v received command: %v\n", s.id, cmd.Timestamp)

	index := s.index.GetAndIncrement()
	instID := pb.InstanceID{
		ReplicaID: s.id,
		Index:     index,
	}

	inst := &instance{
		s: s,
		is: pb.InstanceState{
			View:       s.view,
			InstanceID: instID,
			Status:     pb.InstanceState_Prepared,
			Command:    cmd,
		},
		cCert: newQuorum(s),
	}
	s.log[instID] = inst
	s.logger.Debugf("onRequest inst %v", instID)

	pm := &pb.NormalMessage{
		View:       s.view,
		InstanceID: instID,
		Type:       pb.NormalMessage_Prepare,
		Command:    cmd,
	}

	mBytes := s.marshall(pm)

	s.broadcast(mBytes, true /* sendToSelf */, nil)

	return inst
}
