package hybster

import (
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybster/hybsterpb"
)

func (p *hybster) wrapAndMarshal(m proto.Message) []byte {
	hMsg := pb.WrapHybsterMessage(m)

	mBytes, err := proto.Marshal(hMsg)
	if err != nil {
		p.logger.Panic("unable to marshal")
	}
	return mBytes
}

func (p *hybster) marshalAndSignNormal(m *pb.NormalMessage) ([]byte, []byte) {
	mBytes := p.wrapAndMarshal(m)

	counter := uint64(uint64(m.View<<48) | uint64(m.Order))
	cert := p.certifier.CreateIndependentCounterCertificateVector(mBytes, counter, 0)

	return mBytes, cert[:32]
}

func (p *hybster) sendTo(to peerpb.PeerID, mm, cert []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: cert,
		To:          to,
		From:        p.id,
	}
	p.msgs = append(p.msgs, cm)
}

func (p *hybster) broadcastNormal(m *pb.NormalMessage) {
	mBytes := p.wrapAndMarshal(m)

	counter := uint64(uint64(m.View<<48) | uint64(m.Order))

	p.signDispatcher.Exec(func() []byte {
		return p.certifier.CreateIndependentCounterCertificateVector(mBytes, counter, 0)
	}, func(cert []byte) {
		if m.Type == pb.NormalMessage_Prepare {
			p.omsgs = append(p.omsgs, prPkg{m, mBytes, cert[:32], p.id})
		}
		for _, node := range p.nodes {
			if node != p.id {
				idx := node * 32
				p.sendTo(node, mBytes, cert[idx:idx+32])
			}
		}
	})
}

func (p *hybster) broadcastVC(m *pb.ViewChangeMessage) {
	mBytes := p.wrapAndMarshal(m)

	counter := uint64(m.ToView << 48)
	cert := p.certifier.CreateContinuingCounterCertificate(mBytes, counter, 0)

	// p.logger.Debugf("Signing %v %v %v %v", counter, mHash[:], cert, mBytes)

	p.vmsgs = append(p.vmsgs, vcPkg{m, mBytes, cert, p.id})
	p.broadcast(mBytes, cert)
}

func (p *hybster) broadcastNV(m pb.NewViewMessage) {
	mBytes := p.wrapAndMarshal(&m)

	counter := uint64(m.ToView << 48)
	cert := p.certifier.CreateIndependentCounterCertificate(mBytes, counter, 1)

	p.nvmsgs = append(p.nvmsgs, m)
	p.broadcast(mBytes, cert)
}

func (p *hybster) broadcastNVA(m *pb.NewViewAckMessage) {
	mBytes := p.wrapAndMarshal(m)

	mHash := sha256.Sum256(mBytes)
	cert := p.certifier.CreateContinuingCounterCertificate(mHash[:], 0, 2)

	p.nvamsgs = append(p.nvamsgs, nvaPkg{m, mBytes, cert, p.id})
	p.broadcast(mBytes, cert)
}

func (p *hybster) broadcast(mBytes, cert []byte) {
	for _, node := range p.nodes {
		if node != p.id {
			p.sendTo(node, mBytes, cert)
		}
	}
}

func (p *hybster) ClearMsgs() {
	p.msgs = nil
}
