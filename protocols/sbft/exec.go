package sbft

import pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"

func (s *sbft) exec() {
	for {
		inst, ok := s.log[s.execIdx]
		if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
			break
		}

		s.logger.Debugf("Replica %d execute [s=%d, cmd=%d]\n", s.id, s.execIdx,
			inst.is.Command.Timestamp)

		// inst.is.Command.TraceInfo = inst.traceInfo

		s.Execute(inst.is.Command)
		s.execIdx++

		inst.is.Command = nil
		inst.is.Status = pb.InstanceState_Executed
	}
}
