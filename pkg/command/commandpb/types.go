package commandpb

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/gogo/protobuf/proto"
)

type Key []byte

type OperationResults []OperationResult

// Compare compares the two Keys.
// The result will be 0 if k == k2, -1 if k < k2, and +1 if k > k2.
func (k Key) Compare(k2 Key) int {
	return bytes.Compare(k, k2)
}

// Equal returns whether two Keys are identical.
func (k Key) Equal(o Key) bool {
	return bytes.Equal(k, o)
}

func (k Key) String() string {
	return fmt.Sprintf("%q", []byte(k))
}

// Equal returns whether two Operation are identical.
func (k *KVOp) Equal(o *KVOp) bool {
	if k == o {
		return true
	}
	if o == nil {
		return false
	}
	return bytes.Equal(k.Key, o.Key) &&
		bytes.Equal(k.Value, o.Value) &&
		k.Read == o.Read
}

// Equal returns whether two Operation are identical.
func (k *KVOpResult) Equal(o *KVOpResult) bool {
	if k == o {
		return true
	}
	if o == nil {
		return false
	}
	return bytes.Equal(k.Key, o.Key) &&
		bytes.Equal(k.Value, o.Value) &&
		k.WriteSuccess == o.WriteSuccess
}

// Equal returns whether two Operation are identical.
func (k *Operation) Equal(o *Operation) bool {
	switch t := k.Type.(type) {
	case *Operation_KVOp:
		return t.KVOp.Equal(o.GetKVOp())
	}
	return false
}

// Equal returns whether two OperationResult are identical.
func (k *OperationResult) Equal(o *OperationResult) bool {
	switch t := k.Type.(type) {
	case *OperationResult_KVOpResult:
		return t.KVOpResult.Equal(o.GetKVOpResult())
	}
	return false
}

// Equal returns whether two OperationResult are identical.
func (k OperationResults) Equal(o OperationResults) bool {
	if len(k) != len(o) {
		return false
	}
	for i := range k {
		if !k[i].Equal(&o[i]) {
			return false
		}
	}
	return true
}

// Hash returns the hash of the command.
func (m *Command) Hash() []byte {
	traceInfo := m.TraceInfo
	m.TraceInfo = nil

	cmdBytes, err := proto.Marshal(m)
	if err != nil {
		return nil
	}
	m.TraceInfo = traceInfo

	cmdHash := sha256.Sum256(cmdBytes)
	return cmdHash[:]
}

// Interferes returns whether the two Commands interfere.
func (m *Command) Interferes(o Command) bool {
	return bytes.Equal(m.ConflictKey, o.ConflictKey)
}

func (m *Command) Equal(o *Command) bool {
	if m.Timestamp != o.Timestamp ||
		m.Target != o.Target ||
		!bytes.Equal(m.ConflictKey, o.ConflictKey) {
		return false
	}
	if len(m.Ops) != len(o.Ops) {
		return false
	}
	for i := range m.Ops {
		if !m.Ops[i].Equal(&o.Ops[i]) {
			return false
		}
	}
	return true
}

func (m *CommandResult) Equal(o *CommandResult) bool {
	if m.Timestamp != o.Timestamp || m.Target != o.Target ||
		!bytes.Equal(m.Meta, o.Meta) {
		return false
	}
	return OperationResults(m.OpResults).Equal(o.OpResults)
}
