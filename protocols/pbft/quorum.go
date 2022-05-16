package pbft

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
)

type Quorum struct {
	p *PBFT

	maxView pb.View
	msgs    map[pb.View]map[peerpb.PeerID]*pb.AgreementMessage
}

// newQuorum returns a new Quorum
func newQuorum(p *PBFT) *Quorum {
	return &Quorum{
		p:    p,
		msgs: make(map[pb.View]map[peerpb.PeerID]*pb.AgreementMessage),
	}
}

// log adds id to quorum log records
func (q *Quorum) log(id peerpb.PeerID, m *pb.AgreementMessage) {
	if q.maxView > m.View {
		q.p.logger.Panicf("quorum view is greater %v > %v", q.maxView, m.View)
	}
	q.maxView = m.View

	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]*pb.AgreementMessage)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = m
	} else {
		q.p.logger.Panicf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *Quorum) Majority(m *pb.AgreementMessage) bool {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) <= 2*q.p.f {
			return false
		}

		size := 0
		for _, otherMsg := range msgPairs {
			if m.Equals(otherMsg) {
				size++
			}
		}
		return size > 2*q.p.f
	}
	return false
}
