package discovery

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	pb "github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"google.golang.org/grpc"
)

type Discovery struct {
	*Config

	identifier int32

	msgC       chan packet
	lis        net.Listener
	grpcServer *grpc.Server
	logger     logger.Logger

	regNodes map[peerpb.BasicPeerInfo]peerpb.PeerInfo
	resChans []packet
	sInfo    []peerpb.PeerInfo

	clientIDGen uint32
	leader      peerpb.PeerID

	secretKeys []string
	publicKeys []string
}

type packetType int

const (
	pktRegister packetType = iota + 1
	pktRetrieve
)

type packet struct {
	id      peerpb.BasicPeerInfo
	retC    chan<- bool
	pktType packetType
}

type retPacket struct {
	id   peerpb.BasicPeerInfo
	retC chan<- bool
}

func dieOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func NewDiscoveryServer(portnum int, logger logger.Logger, config *Config) (*Discovery, error) {
	logger.Infof("Discovery provider starting on port %d\n", portnum)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", portnum))
	if err != nil {
		logger.Errorf("Unble to listen on port %d", portnum)
		return nil, err
	}

	d := &Discovery{
		Config:     config,
		identifier: 0,

		msgC:       make(chan packet, config.NodeCount),
		lis:        lis,
		grpcServer: grpc.NewServer(),
		logger:     logger,

		regNodes: make(map[peerpb.BasicPeerInfo]peerpb.PeerInfo),
		resChans: make([]packet, 0, config.NodeCount),

		leader: math.MaxUint64,
	}

	d.readKeys()

	return d, nil
}

func (d *Discovery) Run(ctx context.Context) {
	for {
		select {
		case pkt := <-d.msgC:
			switch pkt.pktType {
			case pktRegister:
				d.processPacket(pkt)
			case pktRetrieve:
				d.retrieveServiceInfo(pkt)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Discovery) Serve(ctx context.Context) error {
	// go d.startHTTP()
	go d.Run(ctx)

	pb.RegisterServiceDiscoveryServer(d.grpcServer, d)
	defer d.Stop()
	return d.grpcServer.Serve(d.lis)
}

func (d *Discovery) Stop() {
	d.grpcServer.Stop()
}

func (d *Discovery) readKeys() {
	if len(d.KeyFile) <= 0 {
		return
	}
	var skeys []string
	var vkeys []string

	file, err := os.Open(d.KeyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	idx := 0
	for idx < 4 && scanner.Scan() {
		vkey := scanner.Text()
		vkeys = append(vkeys, vkey)
		idx++
	}

	// if idx < 4 {
	// 	log.Fatal("too few lines in file")
	// }

	if err := scanner.Err(); err != nil {
		d.logger.Fatal(err)
	}

	for scanner.Scan() {
		skey := scanner.Text()
		skeys = append(skeys, skey)
	}

	if err := scanner.Err(); err != nil {
		d.logger.Fatal(err)
	}

	d.logger.Infof("Read %d secret keys and %d public keys", len(skeys), len(vkeys))

	d.secretKeys = skeys
	d.publicKeys = vkeys
}

func (m *Discovery) startHTTP() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
	dieOnError(err)
	http.Serve(lis, m)
}

func (m *Discovery) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Fix prefix
	t, err := template.ParseFiles(fmt.Sprintf("%v/index.html", ""))
	dieOnError(err)
	info := m.sInfo
	if info == nil {
		info = make([]peerpb.PeerInfo, 0)
	}
	err = t.Execute(w, info)
	dieOnError(err)
}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}

func (d *Discovery) processPacket(pkt packet) error {
	if info, exists := d.regNodes[pkt.id]; exists {
		d.logger.Infof("node already registered %v; received %v\n", info, pkt)
		return nil
	}

	rID := peerpb.PeerID(d.identifier)
	d.identifier++

	if d.leader == math.MaxUint64 && strings.TrimSpace(pkt.id.Region) == d.LeaderRegion {
		d.leader = rID
		d.logger.Infof("Leader set to %d", rID)
	}

	// log.Printf("container info %v", container)
	nodeInfo := peerpb.PeerInfo{
		BasicPeerInfo: pkt.id,
		PeerID:        rID,
	}
	d.logger.Infof("registered node %v\n", nodeInfo)
	d.regNodes[pkt.id] = nodeInfo
	d.resChans = append(d.resChans, pkt)

	if d.identifier == d.NodeCount {
		d.logger.Infof("registered %v nodes...\n", d.NodeCount)
		if d.leader == math.MaxUint64 {
			d.logger.Errorf("No leader set. Target Region is %v", d.LeaderRegion)
		}
		prompt()
		nodes := make([]peerpb.PeerInfo, 0, d.NodeCount)
		for _, info := range d.regNodes {
			nodes = append(nodes, info)
		}
		d.sInfo = nodes
		for _, rpkt := range d.resChans {
			rpkt.retC <- true
		}
		d.resChans = nil
	} else if d.identifier > d.NodeCount {
		d.logger.Errorf("more nodes registered than expected... %v > %v", d.identifier, d.NodeCount)
	}
	return nil
}

func (d *Discovery) retrieveServiceInfo(pkt packet) {
	if d.identifier == d.NodeCount {
		log.Printf("retrieving service info for %v nodes...\n", d.NodeCount)
		pkt.retC <- true
	} else {
		if d.identifier > d.NodeCount {
			d.logger.Errorf("more nodes registered than expected... %v > %v", d.identifier, d.NodeCount)
			return
		}
		d.resChans = append(d.resChans, pkt)
	}
}

// Register is the grpc handler for registering a service
func (d *Discovery) Register(ctx context.Context, id *peerpb.BasicPeerInfo) (*pb.ServerResponse, error) {
	ret := make(chan bool, 1)
	d.msgC <- packet{
		pktType: pktRegister,
		id:      *id,
		retC:    ret,
	}
	select {
	case <-ret:
		res := &pb.ServerResponse{
			PeerConfig: &peerpb.PeerConfig{
				PeerDetails:        d.sInfo,
				ListenPort:         7000,
				LogVerbose:         d.ServerConfig.LogVerbose,
				Algorithm:          d.Algorithm,
				MaxFailures:        d.MaxFailures,
				MaxFastFailures:    d.MaxFastFailures,
				LeaderID:           d.leader,
				SecretKeys:         d.secretKeys,
				PublicKeys:         d.publicKeys,
				EnclavePath:        d.ServerConfig.EnclavePath,
				EnclaveBatchSize:   d.ServerConfig.EnclaveBatchSize,
				DqOBatchSize:       d.ServerConfig.DqOBatchSize,
				CmdBatchSize:       d.ServerConfig.CmdBatchSize,
				CmdBatchTimeout:    d.ServerConfig.CmdBatchTimeout,
				Workers:            d.ServerConfig.Workers,
				WorkersQueueSizes:  d.ServerConfig.WorkersQueueSizes,
				RccAlgorithm:       d.ServerConfig.RccAlgorithm,
				DispelWaitForAllRb: d.ServerConfig.DispelWaitForAllRb,
			},
			PeerID: d.regNodes[*id].PeerID,
		}
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *Discovery) GetServiceInfo(ctx context.Context, info *peerpb.BasicPeerInfo) (*pb.ClientResponse, error) {
	ret := make(chan bool, 1)
	d.msgC <- packet{
		pktType: pktRetrieve,
		retC:    ret,
	}
	select {
	case <-ret:
		id := atomic.AddUint32(&d.clientIDGen, 1)
		d.logger.Infof("registered client %v", info)
		res := &pb.ClientResponse{
			Config: &masterpb.ClientConfig{
				BaseClientConfig: &d.ClientConfig,
				Nodes:            d.sInfo,
				LeaderID:         d.leader,
				Algorithm:        d.Algorithm,
				MaxFailures:      d.MaxFailures,
				MaxFastFailures:  d.MaxFastFailures,
			},
			ClientId: id,
		}
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
