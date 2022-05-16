package hybsterx

import pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"

func (p *hybsterx) exec() {
	for {
		oinst, ok := p.olog[p.oexecIdx]
		if !ok || !oinst.is.IsCommitted() {
			p.logger.Debugf("Replica %d oinst not committed [oseq=%d]\n",
				p.id, p.oexecIdx)
			return
		}

		for _, nextIID := range oinst.is.Instances {
			// nextIID := pb.InstanceID{
			// 	ReplicaID: nextRID,
			// 	Index:     s.execIdx[nextRID],
			// }
			inst, ok := p.log[nextIID]
			if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
				p.logger.Debugf("Replica %d inst not committed [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					p.id, p.oexecIdx, nextIID.PeerID, p.execIdx[nextIID.PeerID], inst)
				return
			}
		}

		for _, nextIID := range oinst.is.Instances {
			// nextIID := pb.InstanceID{
			// 	ReplicaID: nextRID,
			// 	Index:     s.execIdx[nextRID],
			// }
			inst := p.log[nextIID]

			p.logger.Debugf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
				p.id, p.oexecIdx, nextIID.PeerID, p.execIdx[nextIID.PeerID], inst)

			if inst == nil {
				p.logger.Errorf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					p.id, p.oexecIdx, nextIID.PeerID, p.execIdx[nextIID.PeerID], inst)
			}

			p.Execute(inst.is.Command)
			inst.is.Command = nil
			p.execIdx[nextIID.PeerID]++
			inst.is.Status = pb.InstanceState_Executed
		}
		oinst.is.Status = pb.OInstanceState_Executed
		p.oexecIdx++
	}
}
