package destiny

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/destiny/destinypb"
)

type MessageWithSign struct {
	*pb.NormalMessage
	sign []byte
}

type quorum struct {
	s    *destiny
	msgs map[pb.View]map[peerpb.PeerID]MessageWithSign
}

func newQuorum(s *destiny) *quorum {
	return &quorum{
		s:    s,
		msgs: make(map[pb.View]map[peerpb.PeerID]MessageWithSign),
	}
}

func (q *quorum) log(id peerpb.PeerID, m *pb.NormalMessage, sign []byte) {
	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]MessageWithSign)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = MessageWithSign{m, sign}
	} else {
		q.s.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *quorum) Majority(m *pb.NormalMessage) (map[peerpb.PeerID][]byte, bool) {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) <= q.s.f {
			return nil, false
		}

		size := 0
		sigs := make(map[peerpb.PeerID][]byte, len(msgPairs))
		for id, otherMsg := range msgPairs {
			if m.Equals(otherMsg.NormalMessage) {
				sigs[id] = otherMsg.sign
				size++
			}
		}
		return sigs, size > q.s.f
	}
	return nil, false
}

type OMessageWithSign struct {
	*pb.ONormalMessage
	sign []byte
}

type oquorum struct {
	s    *destiny
	msgs map[pb.View]map[peerpb.PeerID]OMessageWithSign
}

func newOQuorum(s *destiny) *oquorum {
	return &oquorum{
		s:    s,
		msgs: make(map[pb.View]map[peerpb.PeerID]OMessageWithSign),
	}
}

func (q *oquorum) log(id peerpb.PeerID, m *pb.ONormalMessage, sign []byte) {
	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]OMessageWithSign)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = OMessageWithSign{m, sign}
	} else {
		q.s.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *oquorum) Majority(m *pb.ONormalMessage) (map[peerpb.PeerID][]byte, bool) {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) <= q.s.f {
			return nil, false
		}

		size := 0
		sigs := make(map[peerpb.PeerID][]byte, len(msgPairs))
		for id, otherMsg := range msgPairs {
			if m.Equals(otherMsg.ONormalMessage) {
				sigs[id] = otherMsg.sign
				size++
			}
		}
		return sigs, size > q.s.f
	}
	return nil, false
}
