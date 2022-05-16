package worker

import (
	"context"
	"math/rand"
	"sync/atomic"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

func (c *ClientWorker) MultiPrimary(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := rand.Intn(len(c.localIDs))
	target := c.localIDs[tIdx]
	if target == int(c.leaderID) {
		target = c.localIDs[(tIdx+1)%len(c.localIDs)]
	}
	req.Target = peerpb.PeerID(target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan *commandpb.CommandResult, c.peerCount)
	errChan := make(chan peerpb.PeerID, c.peerCount)

	c.logger.Debugf("sending request %v to %d", req.Timestamp, req.Target)
	c.conn.SendToAll(ctx, req, resultChan, errChan)

	firstVal := <-resultChan
	for i := 1; i < c.failures+1; i++ {
		res := <-resultChan
		c.logger.Debugf("received %vth response for %v", i, req.Timestamp)
		if !res.Equal(firstVal) {
			panic("VALUES NOT EQUAL")
		}
	}
	return firstVal, nil
}

func (c *ClientWorker) MultiPrimaryRRTarget(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := atomic.AddUint64(c.nextID, 1)
	tIdx = tIdx % uint64(len(c.localIDs))

	req.Target = peerpb.PeerID(c.localIDs[tIdx])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan *commandpb.CommandResult, c.peerCount)
	errChan := make(chan peerpb.PeerID, c.peerCount)

	c.logger.Debugf("sending request %v; target: %d", req.Timestamp, req.Target)
	c.conn.SendToAll(ctx, req, resultChan, errChan)

	firstVal := <-resultChan
	for i := 1; i < c.failures+1; i++ {
		res := <-resultChan
		c.logger.Debugf("received %vth response for %v", i, req.Timestamp)
		if !res.Equal(firstVal) {
			panic("VALUES NOT EQUAL")
		}
	}
	return firstVal, nil
}
