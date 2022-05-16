package hybsterx

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

func (p *hybsterx) wrapAndMarshal(m proto.Message) []byte {
	hMsg := pb.WrapHybsterxMessage(m)

	mBytes, err := proto.Marshal(hMsg)
	if err != nil {
		p.logger.Panic("unable to marshal")
	}
	return mBytes
}

func (p *hybsterx) sendTo(to peerpb.PeerID, mm, cert []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: cert,
		To:          to,
		From:        p.id,
	}
	p.msgs = append(p.msgs, cm)
}

func (p *hybsterx) broadcastNormal(m *pb.NormalMessage) {
	mBytes := p.wrapAndMarshal(m)

	counter := uint64(uint64(m.View<<48)|uint64(m.InstanceID.Index)) + 1
	ctrIdx := int(m.InstanceID.PeerID)

	p.signDispatcher.Exec(func() []byte {
		return p.certifier.CreateIndependentCounterCertificateVector(mBytes, counter, ctrIdx)
	}, func(cert []byte) {
		for _, node := range p.nodes {
			if node != p.id {
				idx := node * 32
				p.sendTo(node, mBytes, cert[idx:idx+32])
			}
		}
	}, int(m.InstanceID.PeerID)%p.signWorkerCount)
}

func (p *hybsterx) broadcastONormal(m *pb.ONormalMessage) {
	mBytes := p.wrapAndMarshal(m)

	counter := uint64(uint64(m.View<<48)|uint64(m.Index)) + 1
	ctrIdx := len(p.nodes)

	p.signDispatcher.Exec(func() []byte {
		return p.certifier.CreateIndependentCounterCertificateVector(mBytes, counter, ctrIdx)
	}, func(cert []byte) {
		for _, node := range p.nodes {
			if node != p.id {
				idx := node * 32
				p.sendTo(node, mBytes, cert[idx:idx+32])
			}
		}
	}, len(p.nodes)%p.signWorkerCount)
}

func (p *hybsterx) ClearMsgs() {
	p.msgs = nil
}
