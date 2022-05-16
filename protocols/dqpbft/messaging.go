package dqpbft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
)

func (d *DQPBFT) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        d.id,
	}
	d.msgs = append(d.msgs, cm)
}

func (d *DQPBFT) sendTo(to peerpb.PeerID, m proto.Message) {
	pbftMsg := pb.WrapDQPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	cert := d.certifier.CreateSignedCertificate(mBytes, 0)
	d.enqueueForSending(to, mBytes, cert)
}

func (d *DQPBFT) broadcast(m proto.Message, sendToSelf bool) {
	pbftMsg := pb.WrapDQPBFTMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	d.signDispatcher.Exec(func() []byte {
		certVector := make([]byte, 0, len(d.peers)*32)
		for range d.peers {
			cert := d.certifier.CreateSignedCertificate(mBytes, 0)
			certVector = append(certVector, cert...)
		}
		return certVector
	}, func(cert []byte) {
		for _, node := range d.peers {
			node := node
			if sendToSelf || node != d.id {
				idx := node * 32
				d.enqueueForSending(node, mBytes, cert[idx:idx+32])
			}
		}
	})
}

func (p *DQPBFT) ClearMsgs() {
	p.msgs = nil
}
