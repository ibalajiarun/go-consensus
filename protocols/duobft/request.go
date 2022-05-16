package duobft

import (
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
)

func (p *duobft) onRequest(cmd *commandpb.Command) *instance {
	p.logger.Debugf("Replica %v received %v\n", p.id, cmd.Timestamp)
	if uint64(p.curView)%uint64(len(p.nodes)) != uint64(p.id) {
		p.logger.Errorf("Not the leader of view %d: %d", p.curView, p.id)
		return nil
	}

	p.index++

	cmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	cmdHash := sha256.Sum256(cmdBytes)

	inst := &instance{
		p: p,
		is: pb.InstanceState{
			View:        p.curView,
			Index:       p.index,
			Status:      pb.InstanceState_Prepared,
			TStatus:     pb.InstanceState_TPrepared,
			Command:     cmd,
			CommandHash: cmdHash[:],
		},

		tcCert: newQuorum(p, p.f+1),
		pcCert: newQuorum(p, 2*p.f+1),
		cCert:  newQuorum(p, 2*p.f+1),
	}
	p.log[p.index] = inst

	pm := &pb.NormalMessage{
		View:        p.curView,
		Index:       p.index,
		Type:        pb.NormalMessage_Prepare,
		Command:     cmd,
		CommandHash: cmdHash[:],
	}

	mBytes := p.marshall(pm)

	p.dispatcher2.Exec(
		func() []byte {
			sign := p.usigSign(mBytes)
			return sign
		},
		func(sign []byte) {
			inst.tcCert.log(p.id, pm)
			inst.pcCert.log(p.id, pm)

			p.broadcast(mBytes, false, sign)
		},
	)

	return inst
}
