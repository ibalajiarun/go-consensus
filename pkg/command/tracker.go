package command

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

// Tracker tracks in-flight Raft proposals.
type Tracker struct {
	logger    logger.Logger
	requests  map[trackerID]chan<- *commandpb.CommandResult
	responses map[trackerID]*commandpb.CommandResult
}

type trackerID struct {
	target peerpb.PeerID
	id     uint64
}

// MakeTracker creates a new proposal Tracker.
func MakeTracker(log logger.Logger) Tracker {
	return Tracker{
		logger:    log,
		requests:  make(map[trackerID]chan<- *commandpb.CommandResult),
		responses: make(map[trackerID]*commandpb.CommandResult),
	}
}

// Register registers a new proposal with the tracker.
func (pr *Tracker) Register(cmd *commandpb.Command, c chan<- *commandpb.CommandResult) bool {
	pr.logger.Debugf("Registering response stream for %d %d", cmd.Target, cmd.Timestamp)
	tid := trackerID{cmd.Target, cmd.Timestamp}

	if res, ok := pr.responses[tid]; ok {
		c <- res
		return false
	}

	if _, ok := pr.requests[tid]; !ok {
		pr.requests[tid] = c
		return true
	}

	return false
}

// Finish informs a tracked proposal that it has completed.
func (pr *Tracker) Finish(cp *commandpb.CommandResult) {
	pr.logger.Debugf("Notifying response stream for %d %d", cp.Target, cp.Timestamp)
	tid := trackerID{cp.Target, cp.Timestamp}
	if ret, ok := pr.requests[tid]; ok {
		delete(pr.requests, tid)
		ret <- cp
	} else {
		pr.responses[tid] = cp
	}
}

// FinishAll informs all tracked proposal that they have completed.
func (pr *Tracker) FinishAll() {
	for id, c := range pr.requests {
		close(c)
		delete(pr.requests, id)
	}
}
