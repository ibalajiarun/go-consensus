package linhybsterpb

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

func (m *NormalMessage) Equals(other *NormalMessage) bool {
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
	if !bytes.Equal(m.Result, other.Result) {
		return false
	}
	return true
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isLinHybsterMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &LinHybsterMessage_Normal{Normal: t}
	case *ResultMessage:
		return &LinHybsterMessage_Result{Result: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapLinHybsterMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapLinHybsterMessage(msg proto.Message) *LinHybsterMessage {
	return &LinHybsterMessage{Type: WrapMessageInner(msg)}
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
