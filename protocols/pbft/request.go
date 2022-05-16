package pbft

import (
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
)

func (p *PBFT) onRequest(cmd *commandpb.Command) *instance {
	p.logger.Debugf("Replica %v received %v\n", p.id, cmd.Timestamp)
	if uint64(p.view)%uint64(len(p.nodes)) != uint64(p.id) {
		p.logger.Errorf("Not the leader of view %d: %d", p.view, p.id)
		return nil
	}

	p.index++

	cmdHash := cmd.Hash()

	inst := &instance{
		p: p,
		is: pb.InstanceState{
			View:        p.view,
			Index:       p.index,
			Status:      pb.InstanceState_PrePrepared,
			Command:     cmd,
			CommandHash: cmdHash[:],
		},
		pCert: newQuorum(p),
		cCert: newQuorum(p),
	}
	p.log[p.index] = inst

	ppm := &pb.AgreementMessage{
		View:        p.view,
		Index:       p.index,
		Type:        pb.AgreementMessage_PrePrepare,
		Command:     cmd,
		CommandHash: cmdHash[:],
	}
	inst.pCert.log(p.id, ppm)
	p.broadcast(ppm, false)

	return inst
}
