package discovery

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"google.golang.org/grpc"
)

type Discovery struct {
	Config

	identifier int32

	msgC       chan packet
	grpcServer *grpc.Server

	peers          map[string]peerpb.PeerInfo
	peersByRegion  map[string][]peerpb.PeerInfo
	cIDGenByRegion sync.Map
	ipMap          map[string]string

	retChans []chan<- bool
	sInfo    []peerpb.PeerInfo

	leader peerpb.PeerID

	secretKeys []string
	publicKeys []string

	ctx       context.Context
	ctxCancel func()

	readyChan chan error

	logger logger.Logger
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

func NewDiscoveryServer(logger logger.Logger, config Config, ipMapping map[string]string) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Discovery{
		Config:     config,
		identifier: 0,

		msgC:       make(chan packet, config.PeerCount),
		grpcServer: grpc.NewServer(),

		peers:         make(map[string]peerpb.PeerInfo),
		peersByRegion: make(map[string][]peerpb.PeerInfo),
		ipMap:         ipMapping,

		retChans: make([]chan<- bool, 0, config.PeerCount),

		leader:    math.MaxUint64,
		ctx:       ctx,
		ctxCancel: cancel,
		logger:    logger,

		readyChan: make(chan error),
	}

	d.readKeys()

	return d
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

func (d *Discovery) Serve(lis net.Listener) error {
	go d.Run(d.ctx)
	masterpb.RegisterServiceDiscoveryServer(d.grpcServer, d)
	return d.grpcServer.Serve(lis)
}

func (d *Discovery) Stop() {
	d.grpcServer.Stop()
	d.ctxCancel()
}

func (d *Discovery) processPacket(pkt packet) error {
	if info, exists := d.peers[pkt.id.PodName]; exists {
		d.logger.Infof("node already registered %v; received %v\n", info, pkt)
		return nil
	}

	rID := peerpb.PeerID(d.identifier)
	d.identifier++

	if d.leader == math.MaxUint64 && strings.TrimSpace(pkt.id.Region) == d.LeaderRegion {
		d.leader = rID
		d.logger.Infof("Leader set to %d", rID)
	}

	if newIP, ok := d.ipMap[fmt.Sprintf("%s/%s", pkt.id.HostMachine, pkt.id.PodIP)]; ok {
		pkt.id.PodIP = newIP
	} else {
		d.logger.Errorf("cannot find mapping IP for %v", pkt.id)
	}
	nodeInfo := peerpb.PeerInfo{
		BasicPeerInfo: pkt.id,
		PeerID:        rID,
	}
	d.logger.Infof("registered node %v\n", nodeInfo)
	d.peers[pkt.id.PodName] = nodeInfo
	d.peersByRegion[pkt.id.Region] = append(d.peersByRegion[pkt.id.Region], nodeInfo)
	d.retChans = append(d.retChans, pkt.retC)

	if d.identifier == d.PeerCount {
		d.logger.Infof("registered %v nodes...\n", d.PeerCount)
		if d.leader == math.MaxUint64 {
			d.readyChan <- fmt.Errorf("no leader set. target Region is %s", d.LeaderRegion)
		} else {
			d.readyChan <- nil
		}
	} else if d.identifier > d.PeerCount {
		d.readyChan <- fmt.Errorf("more nodes registered than expected... %v > %v", d.identifier, d.PeerCount)
	}
	return nil
}

func (d *Discovery) WaitReady(ctx context.Context) error {
	for {
		select {
		case err := <-d.readyChan:
			return err
		case <-ctx.Done():
			return fmt.Errorf("timeout occured: %w", ctx.Err())
		}
	}
}

func (d *Discovery) TriggerClients() {
	nodes := make([]peerpb.PeerInfo, 0, d.PeerCount)
	for _, info := range d.peers {
		nodes = append(nodes, info)
	}
	d.sInfo = nodes
	for _, retC := range d.retChans {
		retC <- true
	}
	d.retChans = nil
}

func (d *Discovery) retrieveServiceInfo(pkt packet) {
	if d.identifier == d.PeerCount {
		d.logger.Debugf("retrieving service info for %v nodes...\n", d.PeerCount)
		pkt.retC <- true
	} else {
		if d.identifier > d.PeerCount {
			d.logger.Errorf("more nodes registered than expected... %v > %v", d.identifier, d.PeerCount)
			return
		}
		d.retChans = append(d.retChans, pkt.retC)
	}
}

// Register is the grpc handler for registering a service
func (d *Discovery) Register(ctx context.Context, id *peerpb.BasicPeerInfo) (*masterpb.ServerResponse, error) {
	ret := make(chan bool, 1)
	d.msgC <- packet{
		pktType: pktRegister,
		id:      *id,
		retC:    ret,
	}
	select {
	case <-ret:
		pod, ok := d.peers[id.PodName]
		if !ok {
			d.logger.Panicf("couldn't find peer id for %v", id)
		}
		res := &masterpb.ServerResponse{
			PeerConfig: &peerpb.PeerConfig{
				PeerDetails:            d.sInfo,
				ListenPort:             7000,
				LogVerbose:             d.ServerConfig.LogVerbose,
				Algorithm:              d.Algorithm,
				MaxFailures:            d.MaxFailures,
				MaxFastFailures:        d.MaxFastFailures,
				LeaderID:               d.leader,
				SecretKeys:             d.secretKeys,
				PublicKeys:             d.publicKeys,
				EnclavePath:            d.ServerConfig.EnclavePath,
				EnclaveBatchSize:       d.ServerConfig.EnclaveBatchSize,
				DqOBatchSize:           d.ServerConfig.DqOBatchSize,
				DqOBatchTimeout:        d.ServerConfig.DqOBatchTimeout,
				Workers:                d.ServerConfig.Workers,
				WorkersQueueSizes:      d.ServerConfig.WorkersQueueSizes,
				CmdBatchSize:           d.ServerConfig.CmdBatchSize,
				CmdBatchTimeout:        d.ServerConfig.CmdBatchTimeout,
				ReqBufThreshold:        d.ServerConfig.ReqBufThreshold,
				ThreshsignFastLagrange: d.ServerConfig.ThreshsignFastLagrange,
				RccAlgorithm:           d.ServerConfig.RccAlgorithm,
				DispelWaitForAllRb:     d.ServerConfig.DispelWaitForAllRb,
				MultiChainDuoBFT:       d.ServerConfig.MultiChainDuoBFTConfig,
			},
			PeerID: pod.PeerID,
		}
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *Discovery) GetServiceInfo(ctx context.Context, info *peerpb.BasicPeerInfo) (*masterpb.ClientResponse, error) {
	ret := make(chan bool, 1)
	d.msgC <- packet{
		pktType: pktRetrieve,
		retC:    ret,
	}
	select {
	case <-ret:
		cIdGenPtr, _ := d.cIDGenByRegion.LoadOrStore(info.Region, new(int32))
		cid := nextClientID(cIdGenPtr.(*int32), d.peersByRegion[info.Region], d.PeerCount)
		d.logger.Infof("registered client %v", info)
		res := &masterpb.ClientResponse{
			Config: &masterpb.ClientConfig{
				BaseClientConfig: &d.ClientConfig,
				Nodes:            d.sInfo,
				LeaderID:         d.leader,
				Algorithm:        d.Algorithm,
				MaxFailures:      d.MaxFailures,
				MaxFastFailures:  d.MaxFastFailures,
			},
			ClientId: uint32(cid),
		}
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *Discovery) readKeys() error {
	if len(d.KeyFile) <= 0 {
		return fmt.Errorf("key file mising")
	}
	var skeys []string
	var vkeys []string

	file, err := os.Open(d.KeyFile)
	if err != nil {
		return fmt.Errorf("unable to open keyfile %v: %w", d.KeyFile, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	idx := 0
	for idx < 4 && scanner.Scan() {
		vkey := scanner.Text()
		vkeys = append(vkeys, vkey)
		idx++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("unable to read keyfile %v: %w", d.KeyFile, err)
	}

	for scanner.Scan() {
		skey := scanner.Text()
		skeys = append(skeys, skey)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("unable to read keyfile %v: %w", d.KeyFile, err)
	}

	d.logger.Infof("Read %d secret keys and %d public keys", len(skeys), len(vkeys))

	d.secretKeys = skeys
	d.publicKeys = vkeys

	return nil
}

func nextClientID(idGen *int32, regionPeers []peerpb.PeerInfo, peerCount int32) int32 {
	idx := atomic.AddInt32(idGen, 1)
	cid := int32(regionPeers[int(idx-1)%len(regionPeers)].PeerID) + (int32(int(idx-1)/len(regionPeers)) * peerCount)
	return cid
}
