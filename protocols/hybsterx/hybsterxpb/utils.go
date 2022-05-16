package hybsterxpb

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Index is the number of an instance slot in a replica's command array.
type Index uint64

func (idx *Index) GetAndIncrement() Index {
	index := *idx
	*idx = *idx + 1
	return index
}

// View is a view
type View uint64

func (m *InstanceState) IsPrepared() bool {
	return m.Status == InstanceState_Prepared
}

func (m *InstanceState) IsCommitted() bool {
	return m.Status == InstanceState_Committed
}

func (m *OInstanceState) IsPrepared() bool {
	return m.Status == OInstanceState_Prepared
}

func (m *OInstanceState) IsCommitted() bool {
	return m.Status == OInstanceState_Committed
}

func (m InstanceID) Equals(other InstanceID) bool {
	return m.PeerID == other.PeerID && m.Index == other.Index
}

func (m *NormalMessage) Equals(other *NormalMessage) bool {
	if m.View != other.View {
		return false
	}
	if !m.InstanceID.Equals(other.InstanceID) {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

func (m *ONormalMessage) Equals(other *ONormalMessage) bool {
	if m.View != other.View {
		return false
	}
	if m.Index != other.Index {
		return false
	}

	return bytes.Equal(m.CommandHash, other.CommandHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isHybsterxMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &HybsterxMessage_Normal{Normal: t}
	case *ONormalMessage:
		return &HybsterxMessage_ONormal{ONormal: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapHybsterxMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapHybsterxMessage(msg proto.Message) *HybsterxMessage {
	return &HybsterxMessage{Type: WrapMessageInner(msg)}
}

func InstancesEquals(a, b []InstanceID) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !v.Equals(b[i]) {
			return false
		}
	}
	return true
}
