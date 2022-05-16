package sbftxpb

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

type Index uint64

type View uint64

func (idx *Index) GetAndIncrement() Index {
	index := *idx
	*idx = *idx + 1
	return index
}

func (m *InstanceState) IsPrepared() bool {
	return m.Status == InstanceState_Prepared
}

func (m *InstanceState) IsCommitted() bool {
	return m.Status == InstanceState_Committed
}

func (m *OInstanceState) IsCommitted() bool {
	return m.Status == OInstanceState_Committed
}

func (m InstanceID) Equals(other InstanceID) bool {
	return m.ReplicaID == other.ReplicaID && m.Index == other.Index
}

func (m *NormalMessage) Equals(other *NormalMessage) bool {
	if m.View != other.View {
		return false
	}
	if !m.InstanceID.Equals(other.InstanceID) {
		return false
	}
	// if len(m.Command) != len(other.Command) {
	// 	return false
	// }
	// for i := 0; i < len(m.Command); i++ {
	// 	if !m.Command[i].Equals(&other.Command[i]) {
	// 		return false
	// 	}
	// }
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

func (m *ResultMessage) Equals(other *ResultMessage) bool {
	if len(m.Result) != len(other.Result) {
		return false
	}
	return bytes.Equal(m.Result, other.Result)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isSBFTxMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &SBFTxMessage_Normal{Normal: t}
	case *ONormalMessage:
		return &SBFTxMessage_ONormal{ONormal: t}
	case *ResultMessage:
		return &SBFTxMessage_Result{Result: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapSBFTxMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapSBFTxMessage(msg proto.Message) *SBFTxMessage {
	return &SBFTxMessage{Type: WrapMessageInner(msg)}
}

func ReplicaIDEquals(a, b []peerpb.PeerID) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
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
