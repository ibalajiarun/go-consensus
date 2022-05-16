package chainhotstuff

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

func (hs *chainhotstuff) enqueueForSending(to peerpb.PeerID, mm []byte, certificate []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: certificate,
		To:          to,
		From:        hs.id,
	}
	hs.msgs = append(hs.msgs, cm)
}

func (s *chainhotstuff) marshall(m proto.Message) []byte {
	hsMsg := pb.WrapHostuffMessage(m)
	mBytes, err := proto.Marshal(hsMsg)
	if err != nil {
		panic("unable to marshall")
	}
	return mBytes
}

func (s *chainhotstuff) sendTo(to peerpb.PeerID, mBytes []byte, sign []byte) {
	s.enqueueForSending(to, mBytes, sign)
}

func (s *chainhotstuff) broadcast(mBytes []byte, sendToSelf bool, sign []byte) {
	for _, peer := range s.peers {
		if sendToSelf || peer != s.id {
			s.enqueueForSending(peer, mBytes, sign)
		}
	}
}

func (hs *chainhotstuff) ClearMsgs() {
	hs.msgs = nil
}
