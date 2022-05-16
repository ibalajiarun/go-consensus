package primepb

import (
	"bytes"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

// Index is the number of an instance slot in a replica's command array.
type Index uint64

// View is a view
type View uint64

func (m *InstanceID) Equals(other InstanceID) bool {
	return m.ReplicaID == other.ReplicaID && m.Index == other.Index
}

func (m *InstanceState) IsPreOrdered() bool {
	return m.Status == InstanceState_PreOrder
}

func (m *InstanceState) IsPOAcked() bool {
	return m.Status == InstanceState_PreOrderAck
}

func (m *InstanceState) IsPOSummarized() bool {
	return m.Status == InstanceState_Summary
}

func (m *InstanceState) Equals(other InstanceState) bool {
	return m.InstanceID.Equals(other.InstanceID) &&
		m.Command.Equal(other.Command) && m.View == other.View
}

func (m *POSummary) Equals(other POSummary) bool {
	if len(m.POSummary) != len(other.POSummary) {
		return false
	}
	for i := range m.POSummary {
		if m.POSummary[i] != other.POSummary[i] {
			return false
		}
	}
	return true
}

func (m *OInstanceState) IsPrePrepared() bool {
	return m.Status == OInstanceState_PrePrepared
}

func (m *OInstanceState) IsPrepared() bool {
	return m.Status == OInstanceState_Prepared
}

func (m *OInstanceState) IsCommitted() bool {
	return m.Status == OInstanceState_Committed
}

func (m *PreOrderMessage) Equals(other *PreOrderMessage) bool {
	if !m.InstanceID.Equals(other.InstanceID) {
		return false
	}
	return bytes.Equal(m.CommandHash, other.CommandHash)
}

func (m *OrderMessage) Equals(other *OrderMessage) bool {
	if m.View != other.View {
		return false
	}
	if m.Index != other.Index {
		return false
	}
	// if len(m.POSummaryMatrix.POSummaryMatrix) != len(other.POSummaryMatrix.POSummaryMatrix) {
	// 	return false
	// }
	// for i := range m.POSummaryMatrix.POSummaryMatrix {
	// 	if !m.POSummaryMatrix.POSummaryMatrix[i].Equals(other.POSummaryMatrix.POSummaryMatrix[i]) {
	// 		return false
	// 	}
	// }
	return bytes.Equal(m.POSummaryMatrixHash, other.POSummaryMatrixHash)
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isPrimeMessage_Type {
	switch t := msg.(type) {
	case *PreOrderMessage:
		return &PrimeMessage_Preorder{Preorder: t}
	case *OrderMessage:
		return &PrimeMessage_Order{Order: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapPBFTMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapPBFTMessage(msg proto.Message) *PrimeMessage {
	return &PrimeMessage{Type: WrapMessageInner(msg)}
}
