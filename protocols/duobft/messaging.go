package duobft

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
)

func (s *duobft) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        s.id,
	}
	s.msgs = append(s.msgs, cm)
}

func (s *duobft) marshall(m proto.Message) []byte {
	duobftMsg := pb.WrapduobftMessage(m)
	mBytes, err := proto.Marshal(duobftMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *duobft) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	s.enqueueForSending(to, mBytes, sign)
}

func (s *duobft) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	// s.logger.Debugf("Broadcasting %v", m)
	// cert := s.certifier.CreateSignedCertificate(mBytes, 0)
	// cert := s.signer.Sign(mBytes, 0)
	for _, node := range s.nodes {
		if sendToSelf || node != s.id {
			s.enqueueForSending(node, mBytes, sign)
		}
	}
}

func (s *duobft) ClearMsgs() {
	s.msgs = nil
}
