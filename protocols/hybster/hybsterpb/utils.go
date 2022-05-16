package hybsterpb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

// Index is the number of an instance slot in a replica's command array.
type Order uint64

// View is a view
type View uint64

func (m *NormalMessage) Equals(other *NormalMessage) bool {
	if m.View != other.View {
		return false
	}
	if m.Order != other.Order {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isHybsterMessage_Type {
	switch t := msg.(type) {
	case *NormalMessage:
		return &HybsterMessage_Normal{Normal: t}
	case *ViewChangeMessage:
		return &HybsterMessage_ViewChange{ViewChange: t}
	case *NewViewMessage:
		return &HybsterMessage_NewView{NewView: t}
	case *NewViewAckMessage:
		return &HybsterMessage_NewViewAck{NewViewAck: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapHybsterMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapHybsterMessage(msg proto.Message) *HybsterMessage {
	return &HybsterMessage{Type: WrapMessageInner(msg)}
}
