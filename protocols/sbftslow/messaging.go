package sbftslow

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbftslow/sbftslowpb"
)

func (s *sbftslow) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        s.id,
	}
	s.msgs = append(s.msgs, cm)
}

func (s *sbftslow) marshall(m proto.Message) []byte {
	sbftMsg := pb.WrapSBFTSlowMessage(m)
	mBytes, err := proto.Marshal(sbftMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *sbftslow) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	s.enqueueForSending(to, mBytes, sign)
}

func (s *sbftslow) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	// s.logger.Debugf("Broadcasting %v", m)
	// cert := s.certifier.CreateSignedCertificate(mBytes, 0)
	// cert := s.signer.Sign(mBytes, 0)
	for _, node := range s.nodes {
		if sendToSelf || node != s.id {
			s.enqueueForSending(node, mBytes, sign)
		}
	}
}

func (s *sbftslow) ClearMsgs() {
	s.msgs = nil
}
