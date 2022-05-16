package hybster

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybster/hybsterpb"
)

func (p *hybster) onRequest(cmd *commandpb.Command) *instance {
	//TODO
	if p.curView != p.stableView {
		panic("Fix this")
	}

	p.logger.Debugf("Replica %v received %v\n", p.id, cmd)
	if uint64(p.stableView)%uint64(len(p.nodes)) != uint64(p.id) {
		panic("not the leader")
	}

	p.slot++

	cmdHash := cmd.Hash()

	inst := &instance{
		p:           p,
		view:        p.curView,
		slot:        p.slot,
		command:     cmd,
		commandHash: cmdHash,
		quorum:      newQuorum(p),
	}
	p.log[inst.slot] = inst

	pr := &pb.NormalMessage{
		View:        inst.view,
		Order:       inst.slot,
		Type:        pb.NormalMessage_Prepare,
		Command:     cmd,
		CommandHash: cmdHash[:],
	}
	inst.quorum.log(p.id, pr)
	inst.prepared = true

	p.broadcastNormal(pr)

	return inst
}
