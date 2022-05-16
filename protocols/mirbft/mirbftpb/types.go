package mirbftpb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

type Index uint64

type Epoch uint64

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
	if m.Epoch != other.Epoch {
		return false
	}
	if m.Index != other.Index {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isMirBFTMessage_Type {
	switch t := msg.(type) {
	case *AgreementMessage:
		return &MirBFTMessage_Agreement{Agreement: t}
	case *EpochChangeMessage:
		return &MirBFTMessage_EpochChange{EpochChange: t}
	case *EpochChangeAckMessage:
		return &MirBFTMessage_EpochChangeAck{EpochChangeAck: t}
	case *NewEpochMessage:
		return &MirBFTMessage_NewEpoch{NewEpoch: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapMirBFTMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapMirBFTMessage(msg proto.Message) *MirBFTMessage {
	return &MirBFTMessage{Type: WrapMessageInner(msg)}
}
