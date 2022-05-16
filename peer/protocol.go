package peer

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

type Protocol interface {
	Step(peerpb.Message)
	Tick()
	Request(*commandpb.Command)
	Callback(ExecCallback)

	MakeReady() Ready
	ClearMsgs()
	ClearExecutedCommands()

	AsyncCallback()
}
