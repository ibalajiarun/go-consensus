package prime

import (
	"sort"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

type IndexSlice []pb.Index

func (is IndexSlice) Len() int {
	return len(is)
}

func (is IndexSlice) Less(i, j int) bool {
	return is[i] < is[j]
}

func (is IndexSlice) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func (p *prime) exec() {
	for {
		oinst, ok := p.olog[p.oexecIdx]
		if !ok || !oinst.is.IsCommitted() {
			p.logger.Debugf("Replica %d oinst not committed [oseq=%d]\n",
				p.id, p.oexecIdx)
			return
		}

		mat := oinst.is.POSummaryMatrix.POSummaryMatrix
		var execInstIDs []pb.InstanceID
		for i := range mat {
			counts := make([]pb.Index, len(mat))
			for j := range mat {
				counts[j] = mat[j].POSummary[i]
			}
			sort.Sort(IndexSlice(counts))
			inst := pb.InstanceID{
				ReplicaID: peerpb.PeerID(i),
				Index:     counts[p.f],
			}
			execInstIDs = append(execInstIDs, inst)
		}

		p.logger.Debug(execInstIDs)

		for _, maxInstID := range execInstIDs {
			rid := maxInstID.ReplicaID
			for i := p.execIdx[rid]; i <= maxInstID.Index; i++ {
				nextIID := pb.InstanceID{
					ReplicaID: rid,
					Index:     i,
				}
				inst, ok := p.log[nextIID]
				if !ok || inst.is.Status < pb.InstanceState_PreOrderAck {
					p.logger.Debugf("Replica %d inst not available [oseq=%d, r=%d, s=%d, cmd=%d]\n",
						p.id, p.oexecIdx, rid, p.execIdx[rid], inst)
					return
				}
			}
		}

		for _, maxInstID := range execInstIDs {
			rid := maxInstID.ReplicaID
			for i := p.execIdx[rid]; i <= maxInstID.Index; i++ {
				nextIID := pb.InstanceID{
					ReplicaID: rid,
					Index:     i,
				}
				inst := p.log[nextIID]
				p.logger.Debugf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					p.id, p.oexecIdx, rid, p.execIdx[rid], inst.is.Command)
				if inst.is.Command == nil {
					p.logger.Errorf("Replica %d execIdx [oseq=%d, r=%d, s=%d, inst=%d]\n",
						p.id, p.oexecIdx, rid, p.execIdx[rid], inst)
				}
				if inst.is.Status != pb.InstanceState_Executed {
					p.Execute(inst.is.Command)
					inst.is.Status = pb.InstanceState_Executed
					inst.is.Command = nil
				}
				p.execIdx[rid]++
			}
		}

		p.logger.Debugf("Replica %d ExecDone %v\n", p.id, oinst.is.Index)
		oinst.is.Status = pb.OInstanceState_Executed
		p.oexecIdx++
	}
}
