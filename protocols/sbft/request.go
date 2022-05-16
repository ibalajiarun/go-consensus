package sbft

import (
	"context"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"
	"github.com/opentracing/opentracing-go"
)

func (s *sbft) onRequest(ctx context.Context, cmd *commandpb.Command) *instance {
	s.logger.Debugf("Replica %v received command ID %v\n", s.id, cmd.Timestamp)

	span, ctx := opentracing.StartSpanFromContext(ctx, "onRequest")
	defer span.Finish()

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
