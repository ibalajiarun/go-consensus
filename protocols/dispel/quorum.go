package dispel

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
)

type quorum struct {
	d    *Dispel
	msgs map[peerpb.PeerID]*pb.RBMessage
}

// newQuorum returns a new Quorum
func newQuorum(d *Dispel) *quorum {
	return &quorum{
		d:    d,
		msgs: make(map[peerpb.PeerID]*pb.RBMessage),
	}
}

// log adds id to quorum log records
func (q *quorum) log(id peerpb.PeerID, m *pb.RBMessage) {
	if _, exists := q.msgs[id]; !exists {
		q.msgs[id] = m
	} else {
		q.d.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *quorum) Majority(m *pb.RBMessage) bool {
	if len(q.msgs) <= 2*q.d.maxFailures {
		return false
	}

	size := 0
	for _, otherMsg := range q.msgs {
		if m.Equals(otherMsg) {
			size++
		}
	}
	return size > 2*q.d.maxFailures
}
