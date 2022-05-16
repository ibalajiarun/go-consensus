package dqsbftslowpb

import "testing"

func TestIndexGetAndIncrement(t *testing.T) {
	index := Index(0)

	if index != 0 {
		t.Errorf("Index is not zero: %v", index)
	}

	newIndex := index.GetAndIncrement()
	if newIndex != 0 {
		t.Errorf("getAndIncrement not working: %v", newIndex)
	}
	if index != 1 {
		t.Errorf("getAndIncrement not working: %v", index)
	}
}
