package dqsbftslow

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqsbftslow/dqsbftslowpb"
)

func (s *dqsbftslow) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        s.id,
	}
	s.msgs = append(s.msgs, cm)
}

func (s *dqsbftslow) marshall(m proto.Message) []byte {
	dMsg := pb.WrapSBFTxMessage(m)
	mBytes, err := proto.Marshal(dMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *dqsbftslow) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	// sdbftMsg := pb.WrapSDBFTMessage(m)
	// mBytes, err := proto.Marshal(sdbftMsg)
	// if err != nil {
	// 	panic("unable to marshall")
	// }

	// cert := s.certifier.CreateSignedCertificate(mBytes, 0)
	s.enqueueForSending(to, mBytes, sign)
}

func (s *dqsbftslow) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	// s.logger.Debugf("Broadcasting %v", m)

	// sdbftMsg := pb.WrapSDBFTMessage(m)
	// mBytes, err := proto.Marshal(sdbftMsg)
	// if err != nil {
	// 	panic("unable to marshall")
	// }

	// cert := s.certifier.CreateSignedCertificate(mBytes, 0)

	for _, node := range s.nodes {
		if sendToSelf || node != s.id {
			s.enqueueForSending(node, mBytes, sign)
		}
	}
}

func (s *dqsbftslow) ClearMsgs() {
	s.msgs = nil
}
