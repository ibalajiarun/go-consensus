package connection

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	transport "github.com/ibalajiarun/go-consensus/transport"
	transpb "github.com/ibalajiarun/go-consensus/transport/transportpb"
	"github.com/ibalajiarun/go-consensus/utils/helper"
	"github.com/ibalajiarun/go-consensus/utils/signer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Connection struct {
	peerConns map[peerpb.PeerID]*peerConn
	signer    *signer.Signer
	logger    logger.Logger
}

func New(peers map[peerpb.PeerID]*transport.ExternalClient, logger logger.Logger) *Connection {
	c := &Connection{
		peerConns: make(map[peerpb.PeerID]*peerConn, len(peers)),
		signer:    signer.NewSigner(),
		logger:    logger,
	}
	for id, client := range peers {
		pc := newPeerConn(id, c, client, logger)
		c.peerConns[id] = pc
	}
	return c
}

func (c *Connection) Run() {
	for _, pc := range c.peerConns {
		go pc.run()
	}
}

func (c *Connection) Send(to peerpb.PeerID,
	ctx context.Context, cmd *commandpb.Command,
	resC chan *commandpb.CommandResult, errC chan peerpb.PeerID,
) error {
	cmdBytes, sig := helper.MarshallAndSign(cmd, c.signer)
	sendCP := &transpb.ClientPacket{
		Message:   cmdBytes,
		Signature: sig,
	}
	pc := c.peerConns[to]
	pc.send(ctx, sendCP, resC, errC)
	return nil
}

func (c *Connection) SendToAll(
	ctx context.Context, cmd *commandpb.Command,
	resC chan *commandpb.CommandResult, errC chan peerpb.PeerID,
) {
	cmdBytes, sig := helper.MarshallAndSign(cmd, c.signer)
	sendCP := &transpb.ClientPacket{
		Message:   cmdBytes,
		Signature: sig,
	}

	// only send the payload to the target replica.
	// other replicas get a command without payload for receiving replies only.
	ops := cmd.Ops
	cmd.Ops = nil
	liteCmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		c.logger.Panicf("unable to marshal command")
	}
	cmd.Ops = ops
	liteSendCP := &transpb.ClientPacket{
		Message: liteCmdBytes,
	}

	doneC := make(chan struct{}, len(c.peerConns))
	for dstID, conn := range c.peerConns {
		if dstID == cmd.Target {
			conn.sendC <- sendPkt{sendCP, ctx, resC, errC, doneC}
		} else {
			conn.sendC <- sendPkt{liteSendCP, ctx, resC, errC, doneC}
		}
	}
	for range c.peerConns {
		<-doneC
	}
}

type sendPkt struct {
	cPkt  *transpb.ClientPacket
	ctx   context.Context
	resC  chan *commandpb.CommandResult
	errC  chan peerpb.PeerID
	doneC chan<- struct{}
}

type peerConn struct {
	id     peerpb.PeerID
	conn   *Connection
	client *transport.ExternalClient
	sendC  chan sendPkt
	sig    chan struct{}
	logger logger.Logger
}

func newPeerConn(
	id peerpb.PeerID,
	conn *Connection,
	cli *transport.ExternalClient,
	logger logger.Logger,
) *peerConn {
	pc := &peerConn{
		id:     id,
		conn:   conn,
		client: cli,
		sendC:  make(chan sendPkt, 1),
		sig:    make(chan struct{}, 1),
		logger: logger,
	}
	return pc
}

func (c *peerConn) signal() {
	select {
	case c.sig <- struct{}{}:
	default:
		// Already signaled.
	}
}

func (c *peerConn) run() error {
	for {
		pkt := <-c.sendC
		pkt.doneC <- struct{}{}
		c.send(pkt.ctx, pkt.cPkt, pkt.resC, pkt.errC)
	}
}

func (c *peerConn) send(ctx context.Context, sendCP *transpb.ClientPacket,
	resC chan *commandpb.CommandResult, errC chan peerpb.PeerID) {
	recvCP, err := c.client.Apply(ctx, sendCP)
	switch status.Code(err) {
	case codes.OK:
		cmdResult := &commandpb.CommandResult{}
		helper.VerifyAndUnmarshall(recvCP.Message, recvCP.Signature, cmdResult, c.conn.signer)
		c.logger.Debugf("Reply received %v", cmdResult.Timestamp)
		resC <- cmdResult
	case codes.Unavailable:
		errC <- c.id
	case codes.Canceled:
	default:
		panic(err)
	}
}
