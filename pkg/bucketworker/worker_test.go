package bucketworker

import "testing"

func TestBucketWorker(t *testing.T) {
	count := 10
	workers := 2

	callbackC := make(chan func(), count)
	d := NewBucketDispatcher(count, workers, callbackC)
	d.Run()

	oneCount := 0
	zeroCount := 0
	for i := 0; i < count; i++ {
		i := i
		d.Exec(func() []byte {
			return []byte{byte(i % workers)}
		}, func(b []byte) {
			if b[0] == 0 {
				zeroCount++
			} else {
				oneCount++
			}
		}, i%workers)
	}

	for cb := range callbackC {
		cb()
		if zeroCount+oneCount == 10 {
			break
		}
	}

	if zeroCount != 5 || oneCount != 5 {
		t.Fatalf("count not matching")
	}
}
