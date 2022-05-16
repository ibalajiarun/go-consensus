package hybsterx

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

func (p *hybsterx) onRequest(cmd *commandpb.Command) *instance {
	p.logger.Debugf("Replica %v received %v\n", p.id, cmd)

	cmdHash := cmd.Hash()

	index := p.index.GetAndIncrement()
	instID := pb.InstanceID{
		PeerID: p.id,
		Index:  index,
	}

	inst := &instance{
		p: p,
		is: pb.InstanceState{
			View:        p.view,
			InstanceID:  instID,
			Status:      pb.InstanceState_Prepared,
			Command:     cmd,
			CommandHash: cmdHash,
		},
		quorum: newQuorum(p),
	}
	p.log[instID] = inst
	p.logger.Debugf("onRequest inst %v", index)

	pr := &pb.NormalMessage{
		View:        p.view,
		InstanceID:  instID,
		Type:        pb.NormalMessage_Prepare,
		Command:     cmd,
		CommandHash: cmdHash,
	}
	inst.quorum.log(p.id, pr)

	p.broadcastNormal(pr)

	if p.isPrimaryAtView(p.id, p.oview) {
		p.onCRequest(pr.InstanceID)
	}

	return inst
}
