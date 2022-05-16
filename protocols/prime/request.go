package prime

import (
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

func (p *prime) onRequest(cmd *commandpb.Command) *instance {
	p.logger.Debugf("Replica %v received %v\n", p.id, cmd.Timestamp)

	idx := p.index
	p.index++
	instID := pb.InstanceID{
		ReplicaID: p.id,
		Index:     idx,
	}

	cmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	cmdHash := sha256.Sum256(cmdBytes)

	inst := &instance{
		p: p,
		is: pb.InstanceState{
			InstanceID:  instID,
			Status:      pb.InstanceState_PreOrder,
			Command:     cmd,
			CommandHash: cmdHash[:],
		},
		pCert: newQuorum(p),
		cCert: newQuorum(p),
	}
	p.log[instID] = inst

	poReq := &pb.PreOrderMessage{
		InstanceID:  instID,
		Type:        pb.PreOrderMessage_PORequest,
		Command:     cmd,
		CommandHash: cmdHash[:],
	}
	inst.pCert.log(p.id, poReq)
	p.broadcast(poReq, false)

	return inst
}
