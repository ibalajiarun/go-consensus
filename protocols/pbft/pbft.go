package pbft

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/worker"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
	"github.com/ibalajiarun/go-consensus/utils"
	"github.com/pkg/errors"
)

var (
	ErrNewViewVerifyFailed = errors.New("NewView Verify Failed")
	ErrNewViewNotPossible  = errors.New("NewView: Log has holes")
)

type PBFT struct {
	id    peerpb.PeerID
	nodes []peerpb.PeerID

	log     map[pb.Index]*instance // log ordered by index
	execute pb.Index               // next execute index number
	active  bool                   // active leader
	view    pb.View                // highest view number
	index   pb.Index               // highest index number

	msgs         []peerpb.Message
	executedCmds []peer.ExecPacket

	logger logger.Logger

	timers map[peer.TickingTimer]struct{}

	certifier utils.Certifier

	viewChanging bool
	changingView pb.View
	vcQuorum     *VCQuorum
	f            int

	signDispatcher *worker.Dispatcher
	callbackC      chan func()
}

func NewPBFT(c *peer.LocalConfig) *PBFT {
	log := make(map[pb.Index]*instance, 1024)
	//log[0] = &instance{}
	p := &PBFT{
		id:      c.ID,
		nodes:   c.Peers,
		log:     log,
		execute: 1,
		timers:  make(map[peer.TickingTimer]struct{}),
		logger:  c.Logger,

		certifier: utils.NewSWCertifier(),

		view:  pb.View(c.LeaderID),
		index: 0,

		viewChanging: false,
		f:            int(c.MaxFailures),
	}
	p.vcQuorum = newVCQuorum(p)
	p.certifier.Init()

	if _, ok := c.Workers["mac_sign"]; !ok {
		p.logger.Panicf("Worker count for mac_sign missing.")
	}
	if _, ok := c.WorkersQueueSizes["mac_sign"]; !ok {
		p.logger.Panicf("Worker queue size for mac_sign missing.")
	}

	p.callbackC = make(chan func(), c.WorkersQueueSizes["mac_sign"])
	p.signDispatcher = worker.NewDispatcher(
		int(c.WorkersQueueSizes["mac_sign"]),
		int(c.Workers["mac_sign"]),
		p.callbackC,
	)
	p.signDispatcher.Run()

	return p
}

func (p *PBFT) Step(message peerpb.Message) {
	pbftMsg := &pb.PBFTMessage{}
	if err := proto.Unmarshal(message.Content, pbftMsg); err != nil {
		panic(err)
	}

	if !p.certifier.VerifyCertificate(message.Content, 0, message.Certificate) {
		panic("Invalid certificate")
	}

	switch t := pbftMsg.Type.(type) {
	case *pb.PBFTMessage_Agreement:
		p.stepAgreement(t.Agreement, message.From)
	case *pb.PBFTMessage_ViewChange:
		p.onViewChange(t.ViewChange, message.From)
	case *pb.PBFTMessage_ViewChangeAck:
		p.onViewChangeAck(t.ViewChangeAck, message.From)
	case *pb.PBFTMessage_NewView:
		p.onNewView(t.NewView, message.From)
	}
}

func (p *PBFT) Callback(message peer.ExecCallback) {
	panic("not implemented")
}

func (p *PBFT) stepAgreement(m *pb.AgreementMessage, from peerpb.PeerID) {
	if p.viewChanging {
		p.logger.Infof("View change in progress. Ignoring %v", m)
		return
	}

	// Check if this node is in the same view as the message
	if m.View != p.view {
		p.logger.Errorf("Views do not match. Has: %v, Received: %v", p.view, m.View)
		return
	}

	p.logger.Debugf("Replica %d ====[%v]====>>> Replica %d\n", from, m.Type, p.id)

	switch m.Type {
	case pb.AgreementMessage_PrePrepare:
		p.onPrePrepare(m, from)
	case pb.AgreementMessage_Prepare:
		p.onPrepare(m, from)
	case pb.AgreementMessage_Commit:
		p.onCommit(m, from)
	default:
		p.logger.Panicf("unexpected Message type: %v", m.Type)
	}
}

func (p *PBFT) Tick() {
	for t := range p.timers {
		t.Tick()
	}
}

func (p *PBFT) Request(command *commandpb.Command) {
	p.onRequest(command)
}

func (p *PBFT) MakeReady() peer.Ready {
	return peer.Ready{
		Messages:        p.msgs,
		OrderedCommands: p.executedCmds,
	}
}

func (p *PBFT) ClearExecutedCommands() {
	p.executedCmds = nil
}

func (p *PBFT) Execute(cmd *commandpb.Command) {
	p.executedCmds = append(p.executedCmds, peer.ExecPacket{Cmd: *cmd})
}

func (p *PBFT) AsyncCallback() {
	for {
		select {
		case cb := <-p.callbackC:
			cb()
		default:
			return
		}
	}
}

func (p *PBFT) Stop() {
	p.certifier.Destroy()
}

var _ peer.Protocol = (*PBFT)(nil)

// Start a view change when this node suspects that the primary is faulty
func (p *PBFT) startViewChange(nextView pb.View) {
	if p.viewChanging && nextView <= p.changingView {
		p.logger.Debugf("Higher view change already in effect %v. Got %v", p.changingView, nextView)
		return
	}

	p.viewChanging = true
	p.changingView = nextView

	// Make P and Q sets
	var PSet []pb.InstanceState
	var QSet []pb.InstanceState
	maxIdx := pb.Index(0)
	for idx, inst := range p.log {
		//if inst, ok := p.log[idx]; ok {
		//p.logger.Debugf("instance processing %v", inst.is)
		if inst.is.IsAtleastPrepared() {
			PSet = append(PSet, inst.is)
		}
		if inst.is.IsAtleastPrePrepared() {
			QSet = append(QSet, inst.is)
		}
		if maxIdx < idx {
			maxIdx = idx
		}
		//} else {
		//	p.logger.Fatalf("Instance missing at %d. Is something wrong?", idx)
		//}
	}

	//p.logger.Debugf("PSet %v", PSet)
	//p.logger.Debugf("QSet %v", QSet)

	vc := &pb.ViewChangeMessage{
		NewView:   nextView,
		LowIndex:  0,
		HighIndex: maxIdx,
		PSet:      PSet,
		QSet:      QSet,
	}
	p.broadcast(vc, true)
}

func (p *PBFT) onViewChange(m *pb.ViewChangeMessage, from peerpb.PeerID) {
	if m.NewView <= p.view || (p.viewChanging && m.NewView < p.changingView) {
		p.logger.Debugf("Ignoring stale view change message %v", m)
		return
	}

	p.logger.Debugf("Replica %d ====[ViewChangeMessage]====>>> Replica %d\n", from, p.id)

	for _, is := range m.PSet {
		if is.View > p.view {
			p.logger.Fatalf("View in PSet entry %v is higher than current view %v", is, p.view)
		}
	}
	for _, is := range m.QSet {
		if is.View > p.view {
			p.logger.Fatalf("View in QSet entry %v is higher than current view %v", is, p.view)
		}
	}

	mBytes, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(mBytes)

	to := peerpb.PeerID(int(m.NewView) % len(p.nodes))
	p.sendTo(to, &pb.ViewChangeAckMessage{
		NewView: m.NewView,
		AckFor:  from,
		Digest:  digest[:],
	})

	p.vcQuorum.addVCMessage(from, m, digest[:])
	if p.isPrimaryAtView(p.id, m.NewView) {
		p.constructNewView(m.NewView)
	}

	if nv, ok := p.vcQuorum.nvMessage[to]; ok {
		p.processNewView(nv, to)
	}

	//p.vcQuorum.addVCMessage(vcSender, m)

	//isLaterView := !p.viewChanging || (p.viewChanging && m.View > p.changingView)
	//if isLaterView && p.vcQuorum.count(m) > p.f {
	//	//TODO: Start View Change here.
	//	return
	//}
	//
	//// TODO: If there are 2f + 1 ViewChange messages and the view change timeout is not already
	//// started, update the timeout and start it
	//
	//if p.isPrimaryAtView(p.id, m.View) {
	//	// TODO: Broadcast NewView message.
	//	if p.vcQuorum.countWithout(m, p.id) >= 2*p.f {
	//		nvm := &pb.AgreementMessage{
	//			View:  m.View,
	//			Index: p.index,
	//			Type:  pb.AgreementMessage_NewView,
	//			//Proof: p.vcQuorum.messages,
	//		}
	//		p.broadcast(nvm)
	//	}
	//}
}

func (p *PBFT) onViewChangeAck(m *pb.ViewChangeAckMessage, from peerpb.PeerID) {
	p.logger.Debugf("Replica %d ====[ViewChangeAckMessage]====>>> Replica %d\n", from, p.id)
	p.vcQuorum.addVCAMessage(from, m)
	p.constructNewView(m.NewView)
}

func (p *PBFT) constructNewView(newView pb.View) {
	var SSet []*pb.ViewChangeMessage
	var nvproof []pb.NewViewProof
	maxIdx := pb.Index(0)
	for _, entry := range p.vcQuorum.messages[newView] {
		//p.logger.Debugf("Processing entry from %d: %v", r, entry)
		if entry.valid {
			nvproof = append(nvproof, pb.NewViewProof{
				AckFor: entry.vcSender,
				Digest: entry.vcDigest,
			})
			SSet = append(SSet, entry.vcMessage)
			//p.logger.Debugf("entry %v", entry.vcMessage.String())
			if hi := entry.vcMessage.HighIndex; maxIdx < hi {
				maxIdx = hi
			}
		} else {
			return
		}
	}
	//p.logger.Debugf("SSet calculated: %v; maxIdx: %v", SSet, maxIdx)

	instances := make(map[pb.Index]pb.InstanceState)
	hasHoles := true
	for idx := pb.Index(1); idx <= maxIdx; idx++ {
		if inst := p.getInstanceAtIndex(SSet, idx); inst != nil {
			//instances = append(instances, *inst)
			instances[idx] = *inst
			hasHoles = false
		} else {
			hasHoles = true
			break
		}
	}

	if !hasHoles {
		p.logger.Infof("New view has been calculated: %v", instances)
		p.broadcast(&pb.NewViewMessage{
			NewView:   newView,
			NvProof:   nvproof,
			Instances: instances,
		}, true)
	}
}

func (p *PBFT) getInstanceAtIndex(SSet []*pb.ViewChangeMessage, index pb.Index) *pb.InstanceState {
	for _, vcm := range SSet {
		for _, is := range vcm.PSet {
			if is.Index == index {
				if p.verifyA1(is, SSet) && p.verifyA2(is, SSet) {
					return &is
				}
				//TODO verify condition B
			}
		}
	}
	return nil
}

func (p *PBFT) verifyA1(is1 pb.InstanceState, SSet []*pb.ViewChangeMessage) bool {
	count := 0
for1:
	for _, vcm := range SSet {
		if vcm.LowIndex < is1.Index {
			for _, is2 := range vcm.PSet {
				if (is2.View < is1.View) || (is1.View == is2.View && bytes.Equal(is1.CommandHash, is2.CommandHash)) {
					continue
				} else {
					continue for1
				}
			}
			count++
		}
	}
	//p.logger.Debugf("Count verifyA1 %d", count)
	return count >= 2*p.f+1
}

func (p *PBFT) verifyA2(is1 pb.InstanceState, SSet []*pb.ViewChangeMessage) bool {
	count := 0
	for _, vcm := range SSet {
	for2:
		for _, is2 := range vcm.QSet {
			//p.logger.Debugf("is1 %v", is1)
			//p.logger.Debugf("is2 %v", is2)
			if is2.View >= is1.View && bytes.Equal(is1.CommandHash, is2.CommandHash) {
				count++
				continue for2
			}
		}
	}
	//p.logger.Debugf("Count verifyA2 %d", count)
	return count >= p.f+1
}

func (p *PBFT) onNewView(m *pb.NewViewMessage, from peerpb.PeerID) {
	if !p.isPrimaryAtView(from, m.NewView) {
		p.logger.Fatalf("New View message received from non-primary %v: %v", from, m)
	}
	p.logger.Debugf("Replica %d ====[NewViewMessage]====>>> Replica %d\n", from, p.id)

	p.vcQuorum.addNVMessage(from, m)
	p.processNewView(m, from)

}

func (p *PBFT) processNewView(m *pb.NewViewMessage, from peerpb.PeerID) {
	if !p.viewChanging && p.view == m.NewView {
		return
	}

	if err := p.verifyNewView(m, from); err != nil {
		if err == ErrNewViewVerifyFailed {
			panic("Verification Failed. Trigger VC Again")
		}
		return
	}

	//Apply NewView Message
	//TODO: Change view here
	p.view = m.NewView
	p.viewChanging = false
	for _, is := range m.Instances {
		inst := &instance{
			p: p,
			is: pb.InstanceState{
				View:    m.NewView,
				Index:   is.Index,
				Status:  pb.InstanceState_PrePrepared,
				Command: is.Command,
			},
			pCert: newQuorum(p),
			cCert: newQuorum(p),
		}
		p.log[is.Index] = inst
		ppm := &pb.AgreementMessage{
			View:        m.NewView,
			Index:       is.Index,
			Type:        pb.AgreementMessage_PrePrepare,
			Command:     is.Command,
			CommandHash: is.CommandHash,
		}
		if !p.isCurrentPrimary(p.id) {
			p.onPrePrepare(ppm, from)
		} else {
			inst.pCert.log(p.id, ppm)
		}
	}
}

func (p *PBFT) verifyNewView(m *pb.NewViewMessage, id peerpb.PeerID) error {
	var SSet []*pb.ViewChangeMessage
	var nvproof []pb.NewViewProof
	maxIdx := pb.Index(0)
	for _, entry := range p.vcQuorum.messages[m.NewView] {
		if entry.valid {
			nvproof = append(nvproof, pb.NewViewProof{
				AckFor: entry.vcSender,
				Digest: entry.vcDigest,
			})
			SSet = append(SSet, entry.vcMessage)
			if hi := entry.vcMessage.HighIndex; maxIdx < hi {
				maxIdx = hi
			}
		}
	}

	p.logger.Debugf("NVM %v", m)
	for idx := pb.Index(1); idx <= maxIdx; idx++ {
		inst := p.getInstanceAtIndex(SSet, idx)
		if inst == nil {
			return ErrNewViewNotPossible
		} else if !inst.Equals(m.Instances[idx]) {
			return ErrNewViewVerifyFailed
		}
	}
	return nil
}

func (p *PBFT) hasPrePrepared(index pb.Index) bool {
	return p.hasStatus(index, pb.InstanceState_PrePrepared)
}

func (p *PBFT) hasExecuted(index pb.Index) bool {
	// TODO: Fix committed to executed
	return p.hasStatus(index, pb.InstanceState_Committed)
}

func (p *PBFT) hasStatus(index pb.Index, status pb.InstanceState_Status) bool {
	if inst, ok := p.log[index]; ok {
		return inst.is.Status == status
	}
	return false
}
