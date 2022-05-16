package prime

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

type quorum struct {
	p    *prime
	msgs map[peerpb.PeerID]*pb.PreOrderMessage
}

// newQuorum returns a new Quorum
func newQuorum(p *prime) *quorum {
	return &quorum{
		p:    p,
		msgs: make(map[peerpb.PeerID]*pb.PreOrderMessage),
	}
}

// log adds id to quorum log records
func (q *quorum) log(id peerpb.PeerID, m *pb.PreOrderMessage) {
	if _, exists := q.msgs[id]; !exists {
		q.msgs[id] = m
	} else {
		q.p.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *quorum) Majority(m *pb.PreOrderMessage) bool {
	if len(q.msgs) <= 2*q.p.f {
		return false
	}

	size := 0
	for _, otherMsg := range q.msgs {
		if m.Equals(otherMsg) {
			size++
		}
	}
	return size > 2*q.p.f
}

type OMessageWithSign struct {
	*pb.OrderMessage
	sign []byte
}

type oquorum struct {
	p    *prime
	msgs map[pb.View]map[peerpb.PeerID]*pb.OrderMessage
}

// newQuorum returns a new Quorum
func newOQuorum(p *prime) *oquorum {
	return &oquorum{
		p:    p,
		msgs: make(map[pb.View]map[peerpb.PeerID]*pb.OrderMessage),
	}
}

// log adds id to quorum log records
func (q *oquorum) log(id peerpb.PeerID, m *pb.OrderMessage) {
	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]*pb.OrderMessage)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = m
	} else {
		q.p.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *oquorum) Majority(m *pb.OrderMessage) bool {
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
