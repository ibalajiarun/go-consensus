package dqpbft

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
)

func (d *DQPBFT) onRequest(cmd *commandpb.Command) *instance {
	d.logger.Debugf("Replica %v received command: %v\n", d.id, cmd.Timestamp)

	index := d.index.GetAndIncrement()
	instID := pb.InstanceID{
		ReplicaID: d.id,
		Index:     index,
	}

	inst := &instance{
		d: d,
		is: pb.InstanceState{
			View:       d.view,
			InstanceID: instID,
			Status:     pb.InstanceState_PrePrepared,
			Command:    cmd,
		},
		pCert: newQuorum(d),
		cCert: newQuorum(d),
	}
	d.log[instID] = inst
	d.logger.Debugf("onRequest inst %v", instID)

	ppm := &pb.AgreementMessage{
		View:       inst.is.View,
		InstanceID: inst.is.InstanceID,
		Type:       pb.AgreementMessage_PrePrepare,
		Command:    cmd,
	}
	logppm := &pb.AgreementMessage{
		View:        inst.is.View,
		InstanceID:  inst.is.InstanceID,
		Type:        pb.AgreementMessage_PrePrepare,
		CommandHash: cmd.Hash(),
	}
	inst.pCert.log(d.id, logppm)
	d.broadcast(ppm, false)

	if d.isPrimaryAtView(d.id, d.oview) {
		d.onCRequest(ppm.InstanceID)
	}

	return inst
}
