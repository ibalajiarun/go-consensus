package linhybster

import (
	pb "github.com/ibalajiarun/go-consensus/protocols/linhybster/linhybsterpb"
)

func (s *linhybster) exec() {
	for {
		inst, ok := s.log[s.execIdx]
		if !ok || !inst.is.IsCommitted() || inst.is.Command == nil {
			break
		}

		s.logger.Debugf("Replica %d execute [s=%d, cmd=%d]\n", s.id, s.execIdx,
			inst.is.Command.Timestamp)

		s.Execute(inst.is.Command)
		s.execIdx++

		inst.is.Command = nil
		inst.is.Status = pb.InstanceState_Executed
	}
}
