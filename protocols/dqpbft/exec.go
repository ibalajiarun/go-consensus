package dqpbft

import (
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
)

func (d *DQPBFT) exec() {
	for {
		oinst, ok := d.olog[d.nextODeliverIndex]
		if !ok || !oinst.is.IsCommitted() {
			d.logger.Debugf("Replica %d oinst not committed [oseq=%d]\n",
				d.id, d.nextODeliverIndex)
			return
		}

		for _, nextIID := range oinst.is.Instances {

			inst, ok := d.log[nextIID]
			if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
				d.logger.Debugf("Replica %d inst not committed [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					d.id, d.nextODeliverIndex, nextIID.ReplicaID, d.nextDeliverIndex[nextIID.ReplicaID], inst)
				return
			}
		}

		for _, nextIID := range oinst.is.Instances {
			inst := d.log[nextIID]

			d.logger.Debugf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
				d.id, d.nextODeliverIndex, nextIID.ReplicaID, d.nextDeliverIndex[nextIID.ReplicaID], inst)

			if inst == nil {
				d.logger.Errorf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					d.id, d.nextODeliverIndex, nextIID.ReplicaID, d.nextDeliverIndex[nextIID.ReplicaID], inst)
			}

			d.enqueueForDelivery(inst.is.Command)
			inst.is.Command = nil
			d.nextDeliverIndex[nextIID.ReplicaID]++
			inst.is.Status = pb.InstanceState_Executed
		}
		oinst.is.Status = pb.OInstanceState_OExecuted
		d.nextODeliverIndex++
	}
}

func (d *DQPBFT) enqueueForDelivery(cmd *commandpb.Command) {
	d.toDeliver = append(d.toDeliver, peer.ExecPacket{Cmd: *cmd})
}

func (d *DQPBFT) ClearExecutedCommands() {
	d.toDeliver = nil
}
