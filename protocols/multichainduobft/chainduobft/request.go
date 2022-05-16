package chainduobft

import "github.com/ibalajiarun/go-consensus/pkg/command/commandpb"

func (duobft *ChainDuoBFT) onRequest(cmd *commandpb.Command) {
	duobft.logger.Debugf("onRequest command %v", cmd.Timestamp)

	duobft.cmdQueue.PushBack(cmd)
	duobft.pacemaker.onRequest()
}
