package mirbft

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/mirbft/mirbftpb"
)

func (mir *MirBFT) onRequest(cmd *commandpb.Command) *instance {
	mir.logger.Debugf("Replica %v received %v\n", mir.id, cmd.Timestamp)
	if _, ok := mir.epochLeaders[mir.epoch][mir.id]; !ok {
		mir.logger.Errorf("Not the leader of view %d: %d; epochLeaders are %v", mir.epoch, mir.id, mir.epochLeaders[mir.epoch])
		return nil
	}

	idx := mir.nextLocalIndex
	mir.nextLocalIndex += pb.Index(len(mir.epochLeaders[mir.epoch]))

	inst := &instance{
		mir: mir,
		is: pb.InstanceState{
			Epoch:   mir.epoch,
			Index:   idx,
			Status:  pb.InstanceState_PrePrepared,
			Command: cmd,
		},
		pCert: newQuorum(mir),
		cCert: newQuorum(mir),
	}
	mir.log[idx] = inst

	ppm := &pb.AgreementMessage{
		Epoch:   inst.is.Epoch,
		Index:   inst.is.Index,
		Type:    pb.AgreementMessage_PrePrepare,
		Command: cmd,
	}
	logppm := &pb.AgreementMessage{
		Epoch:       inst.is.Epoch,
		Index:       inst.is.Index,
		Type:        pb.AgreementMessage_PrePrepare,
		CommandHash: cmd.Hash(),
	}
	inst.pCert.log(mir.id, logppm)
	mir.broadcast(ppm, false)

	return inst
}
