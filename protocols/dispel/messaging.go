package dispel

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
)

func (d *Dispel) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        d.id,
	}
	d.msgs = append(d.msgs, cm)
}

func (d *Dispel) sendTo(to peerpb.PeerID, m proto.Message) {
	pbftMsg := pb.WrapDispelMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	cert := d.certifier.CreateSignedCertificate(mBytes, 0)

	d.enqueueForSending(to, mBytes, cert)
}

func (d *Dispel) broadcast(m proto.Message, sendToSelf bool) {
	pbftMsg := pb.WrapDispelMessage(m)
	mBytes, err := proto.Marshal(pbftMsg)
	if err != nil {
		panic("unable to marshall")
	}

	for _, node := range d.peers {
		node := node
		if sendToSelf || node != d.id {
			d.signDispatcher.Exec(func() []byte {
				return d.certifier.CreateSignedCertificate(mBytes, 0)
			}, func(cert []byte) {
				d.enqueueForSending(node, mBytes, cert)
			})
		}
	}
}

func (d *Dispel) ClearMsgs() {
	d.msgs = nil
}
