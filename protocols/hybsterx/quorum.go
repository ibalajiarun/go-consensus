package hybsterx

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybsterx/hybsterxpb"
)

type Quorum struct {
	p *hybsterx

	maxView pb.View
	msgs    map[pb.View]map[peerpb.PeerID]*pb.NormalMessage
}

// newQuorum returns a new Quorum
func newQuorum(p *hybsterx) *Quorum {
	return &Quorum{
		p:    p,
		msgs: make(map[pb.View]map[peerpb.PeerID]*pb.NormalMessage),
	}
}

// log adds id to quorum log records
func (q *Quorum) log(id peerpb.PeerID, m *pb.NormalMessage) {
	if q.maxView > m.View {
		q.p.logger.Panicf("quorum view is greater %v > %v", q.maxView, m.View)
	}
	q.maxView = m.View

	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]*pb.NormalMessage)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = m
	} else {
		q.p.logger.Fatalf("multiple messages vcSender same replica %d: %v", id, m)
	}
}

func (q *Quorum) Reset() {
	q.msgs = make(map[pb.View]map[peerpb.PeerID]*pb.NormalMessage)
}

// Majority quorum satisfied
func (q *Quorum) Majority(m *pb.NormalMessage) bool {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) <= q.p.f {
			return false
		}

		size := 0
		for _, otherMsg := range msgPairs {
			if m.Equals(otherMsg) {
				size++
			}
		}
		return size > q.p.f
	}
	return false
}

type OQuorum struct {
	p *hybsterx

	maxView pb.View
	msgs    map[pb.View]map[peerpb.PeerID]*pb.ONormalMessage
}

// newOQuorum returns a new Quorum
func newOQuorum(p *hybsterx) *OQuorum {
	return &OQuorum{
		p:    p,
		msgs: make(map[pb.View]map[peerpb.PeerID]*pb.ONormalMessage),
	}
}

// log adds id to quorum log records
func (q *OQuorum) log(id peerpb.PeerID, m *pb.ONormalMessage) {
	if q.maxView > m.View {
		q.p.logger.Panicf("quorum view is greater %v > %v", q.maxView, m.View)
	}
	q.maxView = m.View

	if _, exists := q.msgs[m.View]; !exists {
		q.msgs[m.View] = make(map[peerpb.PeerID]*pb.ONormalMessage)
	}
	if _, exists := q.msgs[m.View][id]; !exists {
		q.msgs[m.View][id] = m
	} else {
		q.p.logger.Fatalf("multiple messages vcSender same replica %d: %v", id, m)
	}
}

func (q *OQuorum) Reset() {
	q.msgs = make(map[pb.View]map[peerpb.PeerID]*pb.ONormalMessage)
}

// Majority quorum satisfied
func (q *OQuorum) Majority(m *pb.ONormalMessage) bool {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) <= q.p.f {
			return false
		}

		size := 0
		for _, otherMsg := range msgPairs {
			if m.Equals(otherMsg) {
				size++
			}
		}
		return size > q.p.f
	}
	return false
}
