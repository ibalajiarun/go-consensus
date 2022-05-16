package pbftpb

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Index is the number of an instance slot in a replica's command array.
type Index uint64

// View is a view
type View uint64

func (m *InstanceState) IsPrePrepared() bool {
	return m.Status == InstanceState_PrePrepared
}

func (m *InstanceState) IsPrepared() bool {
	return m.Status == InstanceState_Prepared
}

func (m *InstanceState) IsCommitted() bool {
	return m.Status == InstanceState_Committed
}

func (m *InstanceState) IsAtleastPrePrepared() bool {
	return m.Status >= InstanceState_PrePrepared
}

func (m *InstanceState) IsAtleastPrepared() bool {
	return m.Status >= InstanceState_Prepared
}

func (m *AgreementMessage) Equals(other *AgreementMessage) bool {
	if m.View != other.View {
		return false
	}
	if m.Index != other.Index {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

func (m InstanceState) Equals(other InstanceState) bool {
	return m.Index == other.Index && bytes.Equal(m.CommandHash, other.CommandHash) && m.View == other.View
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isPBFTMessage_Type {
	switch t := msg.(type) {
	case *AgreementMessage:
		return &PBFTMessage_Agreement{Agreement: t}
	case *ViewChangeMessage:
		return &PBFTMessage_ViewChange{ViewChange: t}
	case *ViewChangeAckMessage:
		return &PBFTMessage_ViewChangeAck{ViewChangeAck: t}
	case *NewViewMessage:
		return &PBFTMessage_NewView{NewView: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapPBFTMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapPBFTMessage(msg proto.Message) *PBFTMessage {
	return &PBFTMessage{Type: WrapMessageInner(msg)}
}
