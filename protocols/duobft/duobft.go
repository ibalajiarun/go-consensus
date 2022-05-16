package duobft

import (
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/usig"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
	"github.com/ibalajiarun/go-consensus/utils/signer"
)

type duobft struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID
	f     int

	log      map[pb.Index]*instance // log ordered by slot
	execute  pb.Index               // next execute slot number
	tExecute pb.Index
	index    pb.Index // highest slot number

	msgs         []peerpb.Message
	executedCmds []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier *usig.UsigEnclave
	signer    *signer.Signer

	stableView pb.View
	curView    pb.View

	dispatcher  *worker.Dispatcher
	dispatcher2 *worker.Dispatcher
	callbackC   chan func()
}

func NewDuoBFT(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Index]*instance, 1024)
	log[0] = &instance{}
	p := &duobft{
		id:       c.ID,
		nodes:    c.Peers,
		f:        int(c.MaxFailures),
		log:      log,
		index:    0,
		execute:  1,
		tExecute: 1,
		timers:   make(map[peer.TickingTimer]struct{}),
		logger:   c.Logger,

		certifier: usig.NewUsigEnclave(c.EnclavePath),
		signer:    signer.NewSigner(),

		curView:    pb.View(c.LeaderID),
		stableView: pb.View(c.LeaderID),
	}

	pkey, err := parsePrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	pubkey, err := parsePublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	pkey.PublicKey = *pubkey
	p.certifier.Init([]byte("This is a MAC Key"), pkey, []byte{0}, 1)

	if _, ok := c.Workers["mac_sign"]; !ok {
		p.logger.Panicf("Worker count for mac_sign missing.")
	}
	if _, ok := c.WorkersQueueSizes["mac_sign"]; !ok {
		p.logger.Panicf("Worker queue size for mac_sign missing.")
	}

	p.callbackC = make(chan func(), c.WorkersQueueSizes["mac_sign"])
	p.dispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		int(c.Workers["mac_sign"]),
		p.callbackC,
	)
	p.dispatcher.Run()

	p.dispatcher2 = worker.NewDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		int(c.Workers["mac_sign"]),
		p.callbackC,
	)
	p.dispatcher2.Run()

	return p
}

func (p *duobft) initTimers() {
	// const checkTimeout = 5000
	// checkTimer := peer.MakeTickingTimer(checkTimeout, func() {
	// 	p.logger.Errorf("Check Timer...")
	// 	for _, instID := range p.pendingInstances {
	// 		inst := p.log[instID]
	// 		if inst.is.TStatus < pb.InstanceState_TCommitted {
	// 			p.logger.Errorf("Instance %v not commited yet: %v; votes: %v", instID, inst.is, inst.tcCert.msgs[0])
	// 		}
	// 	}
	// 	p.pendingInstances = nil
	// 	for instID, inst := range p.log {
	// 		if inst.is.TStatus < pb.InstanceState_TCommitted {
	// 			p.pendingInstances = append(p.pendingInstances, instID)
	// 		}
	// 	}
	// 	p.logger.Errorf("=============================")
	// })
	// p.registerInfiniteTimer(checkTimer)
}

func (p *duobft) registerInfiniteTimer(t peer.TickingTimer) {
	p.timers[t] = struct{}{}
	t.Instrument(func() {
		t.Reset()
	})
	t.Reset()
}

func (p *duobft) Step(mm peerpb.Message) {
	m := &pb.DuoBFTMessage{}
	if err := proto.Unmarshal(mm.Content, m); err != nil {
		panic(err)
	}

	p.logger.Debugf("Received from %v: %v", mm.From, m)

	mHash := sha256.Sum256(mm.Content)

	switch t := m.Type.(type) {
	case *pb.DuoBFTMessage_Normal:
		p.stepNormal(t.Normal, mHash, mm.Content, mm.Certificate, mm.From)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *duobft) stepNormal(m *pb.NormalMessage, hash usig.Digest, content, cert []byte, from peerpb.PeerID) {
	if p.curView != p.stableView {
		p.logger.Infof("View change in progress. Ignoring %v", m)
		return
	}

	if m.View != p.curView {
		p.logger.Fatalf("inconsistent views %v != %v", m.View, p.curView)
	}

	p.dispatcher.Exec(
		func() []byte {
			if m.Type < pb.NormalMessage_Commit {
				if !p.usigVerify(hash, cert) {
					p.logger.Panic("Invalid usig certificate: %v", m)
				}
			} else {
				if !p.normalVerify(hash[:], cert) {
					p.logger.Panicf("Invalid normal certificate: %v", m)
				}
			}
			return nil
		},
		func(_ []byte) {
			switch m.Type {
			case pb.NormalMessage_Prepare:
				p.onPrepare(m, from, cert)
			case pb.NormalMessage_PreCommit:
				p.onPreCommit(m, from, cert)
			case pb.NormalMessage_Commit:
				p.onCommit(m, from)
			default:
				p.logger.Panicf("unexpected Message type: %v", m.Type)
			}
		},
	)
}

func (p *duobft) Callback(message peer.ExecCallback) {
}

func (p *duobft) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *duobft) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *duobft) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.executedCmds,
	}
}

func (p *duobft) ClearExecutedCommands() {
	p.executedCmds = nil
}

func (p *duobft) Execute(cmd *commandpb.Command) {
	p.executedCmds = append(p.executedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (p *duobft) AsyncCallback() {
	for {
		select {
		case cb := <-p.callbackC:
			cb()
		default:
			return
		}
	}
}

func (p *duobft) Stop() {
	p.certifier.Destroy()
}

func (p *duobft) leaderOfView(v pb.View) peerpb.PeerID {
	return peerpb.PeerID(v) % peerpb.PeerID(len(p.nodes))
}

func (p *duobft) hasPrepared(index pb.Index) bool {
	return p.hasStatus(index, pb.InstanceState_Prepared)
}

func (p *duobft) hasCommitted(index pb.Index) bool {
	// TODO: Fix committed to executed
	return p.hasStatus(index, pb.InstanceState_Committed)
}

func (p *duobft) hasExecuted(index pb.Index) bool {
	// TODO: Fix committed to executed
	return p.hasStatus(index, pb.InstanceState_Executed)
}

func (p *duobft) hasTExecuted(index pb.Index) bool {
	// TODO: Fix committed to executed
	return p.hasTStatus(index, pb.InstanceState_TExecuted)
}

func (p *duobft) hasStatus(index pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := p.log[index]; ok {
		return inst.is.Status == status
	}
	return false
}

func (p *duobft) hasTStatus(index pb.Index, status pb.InstanceState_TStatus) bool {
	if inst, ok := p.log[index]; ok {
		return inst.is.TStatus == status
	}
	return false
}
