package mirbft

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/mirbft/mirbftpb"
)

type Quorum struct {
	p *MirBFT

	maxView pb.Epoch
	msgs    map[pb.Epoch]map[peerpb.PeerID]*pb.AgreementMessage
}

// newQuorum returns a new Quorum
func newQuorum(p *MirBFT) *Quorum {
	return &Quorum{
		p:    p,
		msgs: make(map[pb.Epoch]map[peerpb.PeerID]*pb.AgreementMessage),
	}
}

// log adds id to quorum log records
func (q *Quorum) log(id peerpb.PeerID, m *pb.AgreementMessage) {
	if q.maxView > m.Epoch {
		q.p.logger.Panicf("quorum view is greater %v > %v", q.maxView, m.Epoch)
	}
	q.maxView = m.Epoch

	if _, exists := q.msgs[m.Epoch]; !exists {
		q.msgs[m.Epoch] = make(map[peerpb.PeerID]*pb.AgreementMessage)
	}
	if _, exists := q.msgs[m.Epoch][id]; !exists {
		q.msgs[m.Epoch][id] = m
	} else {
		q.p.logger.Panicf("multiple messages vcSender same replica %d", id)
	}
}

// Majority quorum satisfied
func (q *Quorum) Majority(m *pb.AgreementMessage) bool {
	if msgPairs, ok := q.msgs[m.Epoch]; ok {
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
