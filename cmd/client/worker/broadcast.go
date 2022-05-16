package worker

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

func (c *ClientWorker) Broadcast(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := rand.Intn(len(c.localIDs))
	target := c.localIDs[tIdx]

	req.Target = peerpb.PeerID(target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan *commandpb.CommandResult, 1)
	errChan := make(chan peerpb.PeerID, 1)

	c.logger.Debugf("sending Broadcast request %v to %v", req.Timestamp, target)
	c.conn.Send(c.leaderID, ctx, req, resultChan, errChan)

	select {
	case errID := <-errChan:
		return nil, fmt.Errorf("client returned error %v", errID)
	case res := <-resultChan:
		c.logger.Debugf("result received for request %v", req.Timestamp)
		return res, nil
	}
}
