package duobftpb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

// Index is the number of an instance slot in a replica's command array.
type Index uint64

// View is a view
type View uint64

func (m *InstanceState) IsPrepared() bool {
	return m.Status == InstanceState_Prepared
}

func (m *InstanceState) IsPreCommitted() bool {
	return m.Status == InstanceState_PreCommitted
}

func (m *InstanceState) IsCommitted() bool {
	return m.Status == InstanceState_Committed
}

func (m *InstanceState) IsTPrepared() bool {
	return m.TStatus == InstanceState_TPrepared
}

func (m *InstanceState) IsTCommitted() bool {
	return m.TStatus == InstanceState_TCommitted
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

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isDuoBFTMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &DuoBFTMessage_Normal{Normal: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapduobftMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapduobftMessage(msg proto.Message) *DuoBFTMessage {
	return &DuoBFTMessage{Type: WrapMessageInner(msg)}
}
