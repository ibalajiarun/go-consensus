package chainhotstuff

import "github.com/ibalajiarun/go-consensus/pkg/command/commandpb"

func (hs *chainhotstuff) onRequest(cmd *commandpb.Command) {
	hs.logger.Debugf("onRequest command %v", cmd.Timestamp)
	hs.cmdQueue.PushBack(cmd)
	hs.pacemaker.beat()
}
