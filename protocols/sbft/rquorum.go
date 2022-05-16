package sbft

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbft/sbftpb"
)

type rquorum struct {
	s    *sbft
	msgs map[uint64]map[peerpb.PeerID]*pb.ResultMessage
}

func newRQuorum(s *sbft) *rquorum {
	return &rquorum{
		s:    s,
		msgs: make(map[uint64]map[peerpb.PeerID]*pb.ResultMessage),
	}
}

func (q *rquorum) log(from peerpb.PeerID, m *pb.ResultMessage) {
	if _, exists := q.msgs[m.Id]; !exists {
		q.msgs[m.Id] = make(map[peerpb.PeerID]*pb.ResultMessage)
	}
	if _, exists := q.msgs[m.Id][from]; !exists {
		q.msgs[m.Id][from] = m
	} else {
		q.s.logger.Fatalf("multiple messages vcSender same replica %d", from)
	}
}

// Majority quorum satisfied
func (q *rquorum) Majority(m *pb.ResultMessage) bool {
	if msgPairs, ok := q.msgs[m.Id]; ok {
		if len(msgPairs) <= q.s.f {
			return false
		}

		size := 0
		for _, otherMsg := range msgPairs {
			if m.Equals(otherMsg) {
				size++
			}
		}
		return size > q.s.f
	}
	return false
}
