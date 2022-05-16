package sbftslowpb

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/proto"
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
	return bytes.Equal(m.Result, other.Result)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isSBFTMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &SBFTMessage_Normal{Normal: t}
	case *ResultMessage:
		return &SBFTMessage_Result{Result: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapSDBFTMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapSBFTSlowMessage(msg proto.Message) *SBFTMessage {
	return &SBFTMessage{Type: WrapMessageInner(msg)}
}
