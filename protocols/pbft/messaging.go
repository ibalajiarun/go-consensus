package pbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
)

func (p *PBFT) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        p.id,
	}
	p.msgs = append(p.msgs, cm)
}

func (p *PBFT) sendTo(to peerpb.PeerID, m proto.Message) {
	pbftMsg := pb.WrapPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	cert := p.certifier.CreateSignedCertificate(mBytes, 0)
	p.enqueueForSending(to, mBytes, cert)
}

func (p *PBFT) broadcast(m proto.Message, sendToSelf bool) {
	pbftMsg := pb.WrapPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	p.signDispatcher.Exec(func() []byte {
		certVector := make([]byte, 0, len(p.nodes)*32)
		for range p.nodes {
			cert := p.certifier.CreateSignedCertificate(mBytes, 0)
			certVector = append(certVector, cert...)
		}
		return certVector
	}, func(cert []byte) {
		for _, node := range p.nodes {
			node := node
			if sendToSelf || node != p.id {
				idx := node * 32
				p.enqueueForSending(node, mBytes, cert[idx:idx+32])
			}
		}
	})
}

func (p *PBFT) ClearMsgs() {
	p.msgs = nil
}
