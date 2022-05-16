package pbft

import (
	"bytes"
	"fmt"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/pbft/pbftpb"
)

type vcItem struct {
	vcSender    peerpb.PeerID
	vcMessage   *pb.ViewChangeMessage
	vcDigest    []byte
	vcaMessages map[peerpb.PeerID]*pb.ViewChangeAckMessage
	valid       bool
}

type VCQuorum struct {
	messages  map[pb.View]map[peerpb.PeerID]*vcItem
	nvMessage map[peerpb.PeerID]*pb.NewViewMessage
	p         *PBFT
}

// newVCQuorum returns a new VCQuorum
func newVCQuorum(p *PBFT) *VCQuorum {
	return &VCQuorum{
		p:         p,
		messages:  make(map[pb.View]map[peerpb.PeerID]*vcItem),
		nvMessage: make(map[peerpb.PeerID]*pb.NewViewMessage),
	}
}

func (q *VCQuorum) addVCMessage(id peerpb.PeerID, m *pb.ViewChangeMessage, digest []byte) {
	vMsgs, ok := q.messages[m.NewView]
	if !ok {
		vMsgs = make(map[peerpb.PeerID]*vcItem)
		q.messages[m.NewView] = vMsgs
	}
	item, ok := vMsgs[id]
	if !ok {
		item = &vcItem{
			vcSender:    id,
			vcMessage:   m,
			vcDigest:    digest,
			vcaMessages: make(map[peerpb.PeerID]*pb.ViewChangeAckMessage),
			valid:       false,
		}
		q.messages[m.NewView][id] = item
	} else {
		item.vcMessage = m
		item.vcDigest = digest
		item.vcSender = id
		item.valid = len(item.vcaMessages) >= 2*q.p.f && item.vcMessage != nil
	}
	if q.messages[m.NewView][id].vcMessage != m {
		panic("OMG")
	}
}

func (q *VCQuorum) addVCAMessage(id peerpb.PeerID, m *pb.ViewChangeAckMessage) {
	vMsgs, ok := q.messages[m.NewView]
	if !ok {
		vMsgs = make(map[peerpb.PeerID]*vcItem)
		q.messages[m.NewView] = vMsgs
	}
	item, ok := vMsgs[m.AckFor]
	if !ok {
		item = &vcItem{
			vcaMessages: make(map[peerpb.PeerID]*pb.ViewChangeAckMessage),
		}
		vMsgs[m.AckFor] = item
	} else {
		if item.vcMessage != nil && !bytes.Equal(item.vcDigest, m.Digest) {
			q.p.logger.Fatalf("Digest not equal. %v != %v", item.vcDigest, m.Digest)
		}
	}
	item.vcaMessages[id] = m
	item.valid = len(item.vcaMessages) >= 2*q.p.f && item.vcMessage != nil
}

func (q *VCQuorum) addNVMessage(id peerpb.PeerID, m *pb.NewViewMessage) {
	if _, ok := q.nvMessage[id]; ok {
		q.p.logger.Debugf("How many NVs will you get? Existing: %v; Received %v", q.nvMessage, m)
	}
	q.nvMessage[id] = m
}

func (i *vcItem) String() string {
	return fmt.Sprintf("<Sender:%d Valid:%v VCM:%v VCA:%v >", i.vcSender, i.valid, i.vcMessage, i.vcaMessages)
}
