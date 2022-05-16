package worker

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

func (c *ClientWorker) Dester(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := rand.Intn(len(c.localIDs))
	target := c.localIDs[tIdx]

	req.Target = peerpb.PeerID(target)

	ctx, cancel := context.WithCancel(context.Background())
	resultChan := make(chan *commandpb.CommandResult, c.peerCount)
	errChan := make(chan peerpb.PeerID, c.peerCount)

	c.logger.Debugf("sending request %v", req.Timestamp)
	c.conn.SendToAll(ctx, req, resultChan, errChan)

	var firstVal *commandpb.CommandResult
	select {
	case errID := <-errChan:
		cancel()
		return nil, fmt.Errorf("client returned error %v", errID)
	case firstVal = <-resultChan:
	}

	for i := 0; i <= c.failures+1; i++ {
		select {
		case errID := <-errChan:
			cancel()
			return nil, fmt.Errorf("client returned error %v", errID)
		case res := <-resultChan:
			c.logger.Debugf("received %vth response for %v", i, req.Timestamp)
			if !res.Equal(firstVal) {
				panic("VALUES NOT EQUAL")
			}
		}
	}
	cancel()
	return firstVal, nil
}
