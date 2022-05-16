package dqpbftpb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

type Index uint64

type View uint64

func (idx *Index) GetAndIncrement() Index {
	index := *idx
	*idx = *idx + 1
	return index
}

func (m *InstanceState) IsPrePrepared() bool {
	return m.Status == InstanceState_PrePrepared
}

func (m *InstanceState) IsPrepared() bool {
	return m.Status == InstanceState_Prepared
}

func (m *InstanceState) IsCommitted() bool {
	return m.Status == InstanceState_Committed
}

func (m *OInstanceState) IsPrePrepared() bool {
	return m.Status == OInstanceState_OPrePrepared
}

func (m *OInstanceState) IsPrepared() bool {
	return m.Status == OInstanceState_OPrepared
}

func (m *OInstanceState) IsCommitted() bool {
	return m.Status == OInstanceState_OCommitted
}

func (m InstanceID) Equals(other InstanceID) bool {
	return m.ReplicaID == other.ReplicaID && m.Index == other.Index
}

func (m *AgreementMessage) Equals(other *AgreementMessage) bool {
	if m.View != other.View {
		return false
	}
	if !m.InstanceID.Equals(other.InstanceID) {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

func (m *OAgreementMessage) Equals(other *OAgreementMessage) bool {
	if m.View != other.View {
		return false
	}
	if m.Index != other.Index {
		return false
	}

	return bytes.Equal(m.CommandHash, other.CommandHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isDQPBFTMessage_Type {
	switch t := msg.(type) {
	case *AgreementMessage:
		return &DQPBFTMessage_Agreement{Agreement: t}
	case *OAgreementMessage:
		return &DQPBFTMessage_OAgreement{OAgreement: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapDQPBFTMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapDQPBFTMessage(msg proto.Message) *DQPBFTMessage {
	return &DQPBFTMessage{Type: WrapMessageInner(msg)}
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
