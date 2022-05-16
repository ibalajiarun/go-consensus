package commandpb

import "testing"

func TestOperationEquals(t *testing.T) {
	op1 := &Operation{
		Type: &Operation_KVOp{
			KVOp: &KVOp{
				Key:   []byte("key"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	op2 := &Operation{
		Type: &Operation_KVOp{
			KVOp: &KVOp{
				Key:   []byte("key"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	op3 := &Operation{
		Type: &Operation_KVOp{
			KVOp: &KVOp{
				Key:   []byte("key-2"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	if !op1.Equal(op2) {
		t.Fatalf("ops not equal")
	}
	if op1.Equal(op3) {
		t.Fatalf("ops not equal")
	}
}

func TestOperationResultEquals(t *testing.T) {
	op1 := &OperationResult{
		Type: &OperationResult_KVOpResult{
			KVOpResult: &KVOpResult{
				Key:   []byte("key"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	op2 := &OperationResult{
		Type: &OperationResult_KVOpResult{
			KVOpResult: &KVOpResult{
				Key:   []byte("key"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	op3 := &OperationResult{
		Type: &OperationResult_KVOpResult{
			KVOpResult: &KVOpResult{
				Key:   []byte("key-2"),
				Value: []byte("random_value_for_testing"),
			},
		},
	}
	op4 := &OperationResult{}
	if !op1.Equal(op2) {
		t.Fatalf("ops not equal")
	}
	if op1.Equal(op3) {
		t.Fatalf("ops not equal")
	}
	if op1.Equal(op4) {
		t.Fatalf("ops not equal")
	}
}
