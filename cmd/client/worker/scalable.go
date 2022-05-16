package worker

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/opentracing/opentracing-go"
)

func (c *ClientWorker) SBFT(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	req.Target = peerpb.PeerID(c.leaderID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "sbft_request")
	defer span.Finish()

	resultChan := make(chan *commandpb.CommandResult, 1)
	errChan := make(chan peerpb.PeerID, 1)

	c.logger.Debugf("sending request %v", req.Timestamp)
	c.conn.Send(c.leaderID, ctx, req, resultChan, errChan)

	select {
	case errID := <-errChan:
		return nil, fmt.Errorf("client returned error %v", errID)
	case res := <-resultChan:
		return res, nil
	}
}

func (c *ClientWorker) ThresholdMultiPrimary(
	req *commandpb.Command,
) (*commandpb.CommandResult, error) {
	tIdx := rand.Intn(len(c.localIDs))
	target := c.localIDs[tIdx]
	for target == int(c.leaderID) {
		tIdx = rand.Intn(len(c.localIDs))
		target = c.localIDs[tIdx]
	}

	req.Target = peerpb.PeerID(target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan *commandpb.CommandResult, 1)
	errChan := make(chan peerpb.PeerID, 1)

	c.logger.Debugf("sending Destiny request %v to %v", req.Timestamp, target)
	c.conn.Send(req.Target, ctx, req, resultChan, errChan)

	select {
	case errID := <-errChan:
		// TODO: Check error and stop further requests.
		return nil, fmt.Errorf("client returned error %v", errID)
	case res := <-resultChan:
		c.logger.Debugf("result received for request %v", req.Timestamp)
		return res, nil
	}
}
