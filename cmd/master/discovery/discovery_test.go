package discovery

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

func TestRegistration(t *testing.T) {
	count := 200
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &Config{
		NodeCount: int32(count),
	}
	logger := logger.NewDefaultLogger()

	m, err := NewDiscoveryServer(1234, logger, cfg)
	if err != nil {
		t.Fatalf("Unable to create server %v", err)
	}

	go m.Run(ctx)

	var wg sync.WaitGroup
	for index := 0; index < count; index++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			nid := &peerpb.BasicPeerInfo{PodName: fmt.Sprintf("%012d", id)}
			_, err := m.Register(ctx, nid)
			if err != nil {
				t.Errorf("Unable to register node %v", nid)
			}
			// t.Logf("Service List Received with %v services", len(services.Nodes))
		}(index)
	}
	wg.Wait()
}
