package duobft

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
)

type Quorum struct {
	p *duobft

	maxView pb.View
	msgs    map[pb.View]map[peerpb.PeerID]*pb.NormalMessage
	qSize   int
}

// newQuorum returns a new Quorum
func newQuorum(p *duobft, qSize int) *Quorum {
	return &Quorum{
		p:     p,
		msgs:  make(map[pb.View]map[peerpb.PeerID]*pb.NormalMessage),
		qSize: qSize,
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
		q.p.logger.Fatalf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *Quorum) Majority(m *pb.NormalMessage) bool {
	if msgPairs, ok := q.msgs[m.View]; ok {
		if len(msgPairs) < q.qSize {
			return false
		}

		size := 0
		for _, otherMsg := range msgPairs {
			if m.Equals(otherMsg) {
				size++
			}
		}
		return size >= q.qSize
	}
	return false
}
