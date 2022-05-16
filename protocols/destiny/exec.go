package destiny

import (
	pb "github.com/ibalajiarun/go-consensus/protocols/destiny/destinypb"
)

func (s *destiny) exec() {
	for {
		oinst, ok := s.olog[s.oexecIdx]
		if !ok || !oinst.is.IsCommitted() {
			s.logger.Debugf("Replica %d oinst not committed [oseq=%d]\n",
				s.id, s.oexecIdx)
			return
		}

		for _, nextIID := range oinst.is.Instances {
			// nextIID := pb.InstanceID{
			// 	ReplicaID: nextRID,
			// 	Index:     s.execIdx[nextRID],
			// }
			inst, ok := s.log[nextIID]
			if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
				s.logger.Debugf("Replica %d inst not committed [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					s.id, s.oexecIdx, nextIID.ReplicaID, s.execIdx[nextIID.ReplicaID], inst)
				return
			}
		}

		for _, nextIID := range oinst.is.Instances {
			// nextIID := pb.InstanceID{
			// 	ReplicaID: nextRID,
			// 	Index:     s.execIdx[nextRID],
			// }
			inst := s.log[nextIID]

			s.logger.Debugf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
				s.id, s.oexecIdx, nextIID.ReplicaID, s.execIdx[nextIID.ReplicaID], inst)

			if inst == nil {
				s.logger.Errorf("Replica %d execIdx [oseq=%d, r=%d, s=%d, cmd=%d]\n",
					s.id, s.oexecIdx, nextIID.ReplicaID, s.execIdx[nextIID.ReplicaID], inst)
			}

			s.Execute(inst.is.Command)
			inst.is.Command = nil
			s.execIdx[nextIID.ReplicaID]++
			inst.is.Status = pb.InstanceState_Executed
		}
		oinst.is.Status = pb.OInstanceState_Executed
		s.oexecIdx++
	}
}
