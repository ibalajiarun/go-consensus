package prime

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

func (p *prime) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        p.id,
	}
	p.msgs = append(p.msgs, cm)
}

func (p *prime) sendTo(to peerpb.PeerID, m proto.Message) {
	pbftMsg := pb.WrapPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	cert := p.certifier.CreateSignedCertificate(mBytes, 0)
	p.enqueueForSending(to, mBytes, cert)
}

func (p *prime) broadcast(m proto.Message, sendToSelf bool) {
	pbftMsg := pb.WrapPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	for _, node := range p.nodes {
		node := node
		if sendToSelf || node != p.id {
			p.signDispatcher.Exec(func() []byte {
				return p.certifier.CreateSignedCertificate(mBytes, 0)
			}, func(cert []byte) {
				p.enqueueForSending(node, mBytes, cert)
			})
		}
	}
}

func (p *prime) ClearMsgs() {
	p.msgs = nil
}
