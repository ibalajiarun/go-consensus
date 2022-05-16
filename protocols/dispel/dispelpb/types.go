package dispelpb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

type Epoch uint64

type Round uint64

func (s *RBState) HasReceived() bool {
	return s.Status == RBState_Received
}

func (s *RBState) HasEchoed() bool {
	return s.Status == RBState_Echoed
}

func (s *RBState) HasReadied() bool {
	return s.Status == RBState_Readied
}

func (m *RBMessage) Equals(other *RBMessage) bool {
	if m.EpochNum != other.EpochNum {
		return false
	}
	if m.PeerID != other.PeerID {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isDispelMessage_Type {
	switch t := msg.(type) {
	case *RBMessage:
		return &DispelMessage_Broadcast{Broadcast: t}
	case *ConsensusMessage:
		return &DispelMessage_Consensus{Consensus: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapDispelMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapDispelMessage(msg proto.Message) *DispelMessage {
	return &DispelMessage{Type: WrapMessageInner(msg)}
}
