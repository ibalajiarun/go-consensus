package worker

import (
	"context"
	"math/rand"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

func (c *ClientWorker) Prime(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := rand.Intn(len(c.localIDs))
	target := c.localIDs[tIdx]

	req.Target = peerpb.PeerID(target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan *commandpb.CommandResult, c.peerCount)
	errChan := make(chan peerpb.PeerID, c.peerCount)

	c.logger.Debugf("sending request %v", req.Timestamp)
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
