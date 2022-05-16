package worker

// func (c *ClientWorker) DuoBFT(
// 	ctx context.Context, key, value []byte, id uint64,
// ) (*commandpb.CommandResult, error) {
// 	var fast []byte
// 	if rand.Intn(100) < c.fastResponse {
// 		fast = []byte{1}
// 	}

// 	req := &transpb.KVRequest{
// 		RequestID: id,
// 		Key:       key,
// 		Value:     value,
// 		Target:    peerpb.PeerID(leaderID),
// 		Metadata:  fast,
// 	}

// 	ctx, cancel := context.WithCancel(ctx)
// 	resultChan := make(chan *commandpb.CommandResult, len(c.serverSet))
// 	errChan := make(chan int, len(c.serverSet))

// 	c.logger.Debugf("sending request %v", req.Timestamp)
// 	for i := range c.serverSet {
// 		c.sendC[i] <- payload{ctx: ctx, req: req, resC: resultChan, errC: errChan}
// 	}

// 	firstVal := <-resultChan
// 	for i := 0; i <= (len(c.serverSet)-1)/3; i++ {
// 		res := <-resultChan
// 		c.logger.Debugf("received %vth response for %v", i, req.Timestamp)
// 		if !reflect.DeepEqual(res, firstVal) {
// 			panic("VALUES NOT EQUAL")
// 		}
// 	}
// 	cancel()
// 	return firstVal, nil
// }
