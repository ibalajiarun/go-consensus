package chainduobft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

func (d *ChainDuoBFT) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        d.id,
	}
	d.msgs = append(d.msgs, cm)
}

func (s *ChainDuoBFT) marshall(m proto.Message) []byte {
	hsMsg := pb.WrapChainDuoBFTMessage(m)
	mBytes, err := proto.Marshal(hsMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *ChainDuoBFT) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	s.enqueueForSending(to, mBytes, sign)
}

func (d *ChainDuoBFT) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	for _, peer := range d.peers {
		if sendToSelf || peer != d.id {
			d.enqueueForSending(peer, mBytes, sign)
		}
	}
}

func (d *ChainDuoBFT) ClearMsgs() {
	d.msgs = nil
}
