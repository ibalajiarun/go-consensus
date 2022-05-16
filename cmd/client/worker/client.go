package worker

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ibalajiarun/go-consensus/cmd/client/connection"
	"github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

type Payload struct {
	Ctx  context.Context
	Req  *commandpb.Command
	ResC chan *commandpb.CommandResult
	ErrC chan int
}

type ClientWorker struct {
	seqNum uint64
	conn   *connection.Connection
	logger logger.Logger

	idPrefix     uint64
	issueRequest func(*commandpb.Command) (*commandpb.CommandResult, error)

	peerCount          int
	reqCount           int
	payloadSize        int
	batchSize          int
	conflictPct        int
	localIDs           []int
	leaderID           peerpb.PeerID
	failures           int
	fastReponsePercent int

	nextID *uint64

	metricC chan<- float64
}

func NewClientWorker(
	conn *connection.Connection,
	logger logger.Logger,
	config *masterpb.ClientConfig,
	appID, cID uint32,
	localIDs []int,
	metricC chan float64,
	nextIDPtr *uint64,
) *ClientWorker {
	return &ClientWorker{
		seqNum: 0,
		conn:   conn,
		logger: logger,

		idPrefix:           (uint64(appID) << 48) | (uint64(cID) << 32),
		peerCount:          len(config.Nodes),
		reqCount:           int(config.TotalRequests),
		payloadSize:        int(config.RequestPayloadSize),
		batchSize:          int(config.RequestOpsBatchSize),
		conflictPct:        int(config.ConflictPercent),
		leaderID:           config.LeaderID,
		localIDs:           localIDs,
		failures:           int(config.MaxFailures),
		fastReponsePercent: int(config.FastResponsePercent),

		metricC: metricC,

		nextID: nextIDPtr,
	}
}

func (c *ClientWorker) Init(
	reqFunc func(*commandpb.Command) (*commandpb.CommandResult, error),
) {
	c.issueRequest = reqFunc
}

func (c *ClientWorker) Run() {
	for i := 0; i < int(c.reqCount); i++ {
		rid := c.idPrefix | uint64(i)

		value := make([]byte, c.payloadSize*int(c.batchSize))
		rand.Read(value)

		conflict := c.conflictPct > 0 && rand.Intn(100) < int(c.conflictPct)
		fastResponse := c.fastReponsePercent > 0 && rand.Intn(100) < int(c.fastReponsePercent)

		key := fmt.Sprintf("key-%d", c.idPrefix)
		if conflict {
			key = "conflicting-key"
		}
		keyBytes := []byte(key)

		ops := make([]commandpb.Operation, c.batchSize)
		for i := range ops {
			ops[i] = commandpb.Operation{
				Type: &commandpb.Operation_KVOp{
					KVOp: &commandpb.KVOp{
						Key:   keyBytes,
						Value: value[i*c.payloadSize : (i+1)*c.payloadSize],
						Read:  false,
					},
				},
			}
		}

		var meta []byte
		if fastResponse {
			meta = []byte{0}
		}

		req := &commandpb.Command{
			Timestamp: rid,
			Ops:       ops,
			Meta:      meta,
		}

		start := time.Now()
		_, err := c.issueRequest(req)
		elapsed := time.Since(start)
		if err != nil {
			log.Println(err)
			continue
		}
		c.metricC <- float64(elapsed.Milliseconds())
	}
}
