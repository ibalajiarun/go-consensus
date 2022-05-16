package mirbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/mirbft/mirbftpb"
)

func (mir *MirBFT) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        mir.id,
	}
	mir.msgs = append(mir.msgs, cm)
}

func (mir *MirBFT) sendTo(to peerpb.PeerID, m proto.Message) {
	pbftMsg := pb.WrapMirBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	cert := mir.certifier.CreateSignedCertificate(mBytes, 0)
	mir.enqueueForSending(to, mBytes, cert)
}

func (p *MirBFT) broadcast(m proto.Message, sendToSelf bool) {
	pbftMsg := pb.WrapMirBFTMessage(m)
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

func (p *MirBFT) ClearMsgs() {
	p.msgs = nil
}
