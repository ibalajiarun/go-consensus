package minbft

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/enclaves/usig"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	pb "github.com/ibalajiarun/go-consensus/protocols/minbft/minbftpb"
)

type minbft struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID
	f     int

	log     map[pb.Order]*instance // log ordered by slot
	execute pb.Order               // next execute slot number
	active  bool                   // active leader
	slot    pb.Order               // highest slot number

	msgs         []peerpb.Message
	executedCmds []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier *usig.UsigEnclave

	stableView pb.View
	curView    pb.View
}

func NewMinBFT(c *peer.LocalConfig) peer.Protocol {
	log := make(map[pb.Order]*instance, 1024)
	log[0] = &instance{}
	p := &minbft{
		id:      c.ID,
		nodes:   c.Peers,
		f:       int(c.MaxFailures),
		log:     log,
		slot:    0,
		execute: 1,
		timers:  make(map[peer.TickingTimer]struct{}),
		logger:  c.Logger,

		certifier: usig.NewUsigEnclave(c.EnclavePath),

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
	return p
}

func (p *minbft) Step(mm peerpb.Message) {
	m := &pb.MinBFTMessage{}
	if err := proto.Unmarshal(mm.Content, m); err != nil {
		panic(err)
	}

	p.logger.Debugf("Received from %v: %v", mm.From, m)

	mHash := sha256.Sum256(mm.Content)

	switch t := m.Type.(type) {
	case *pb.MinBFTMessage_Normal:
		p.stepNormal(t.Normal, mHash, mm.Content, mm.Certificate, mm.From)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *minbft) stepNormal(m *pb.NormalMessage, hash usig.Digest, content, cert []byte, from peerpb.PeerID) {
	if p.curView != p.stableView {
		p.logger.Infof("View change in progress. Ignoring %v", m)
		return
	}

	counter := binary.LittleEndian.Uint64(cert[len(cert)-8:])
	if !p.certifier.VerifyUIMac(hash, cert[:len(cert)-8], counter) {
		p.logger.Errorf("Invalid certificate: Counter: %d", counter)
		return
	}

	if m.View != p.curView {
		p.logger.Fatalf("inconsistent views %v != %v", m.View, p.curView)
	}

	switch m.Type {
	case pb.NormalMessage_Prepare:
		p.onPrepare(m, from)
	case pb.NormalMessage_Commit:
		p.onCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *minbft) Callback(message peer.ExecCallback) {
}

func (p *minbft) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *minbft) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *minbft) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.executedCmds,
	}
}

func (p *minbft) ClearExecutedCommands() {
	p.executedCmds = nil
}

func (p *minbft) Execute(cmd *commandpb.Command) {
	p.executedCmds = append(p.executedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (p *minbft) AsyncCallback() {

}

func (p *minbft) Stop() {
	p.certifier.Destroy()
}

func (p *minbft) hasPrepared(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		p.logger.Debugf("inst %v", inst)
		return inst.prepared
	}
	return false
}

func (p *minbft) hasCommitted(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		return inst.prepared && inst.commit
	}
	return false
}

func (p *minbft) hasExecuted(index pb.Order) bool {
	if inst, ok := p.log[index]; ok {
		return inst.prepared && inst.commit && inst.executed
	}
	return false
}

func (p *minbft) leaderOfView(v pb.View) peerpb.PeerID {
	return peerpb.PeerID(v) % peerpb.PeerID(len(p.nodes))
}
