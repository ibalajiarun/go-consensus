package peer

import (
	"sync"
	"sync/atomic"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

const propBufCap = 1024

type reqBuf struct {
	mu        sync.RWMutex
	full      sync.Cond
	b         [propBufCap]reqBufElem
	i         int32
	threshold int32
}

type reqBufElem struct {
	cmd *commandpb.Command
	c   chan<- *commandpb.CommandResult
}

func (b *reqBuf) init(threshold int32) {
	b.threshold = threshold
	b.full.L = b.mu.RLocker()
}

func (b *reqBuf) add(e reqBufElem) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for {
		n := atomic.AddInt32(&b.i, 1)
		i := int(n - 1)
		if i < len(b.b) {
			b.b[i] = e
			return
		}
		b.full.Wait()
	}
}

func (b *reqBuf) flush(f func([]reqBufElem), force bool) {
	if atomic.LoadInt32(&b.i) < b.threshold && !force {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	i := int(b.i)
	if i >= len(b.b) {
		i = len(b.b)
		b.full.Broadcast()
	}
	if i > 0 {
		f(b.b[:i])
		b.i = 0
	}
}
