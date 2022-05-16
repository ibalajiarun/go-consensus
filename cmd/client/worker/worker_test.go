package worker

import (
	"testing"

	"github.com/ibalajiarun/go-consensus/cmd/master/masterpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

func TestIdPrefix(t *testing.T) {
	config := &masterpb.ClientConfig{
		BaseClientConfig: &masterpb.BaseClientConfig{
			TotalRequests: 10,
		},
	}
	var id uint64
	worker := NewClientWorker(nil, nil, config, 0x1, 0x100, nil, make(chan float64, int(config.TotalRequests)), &id)
	if worker.idPrefix != (0x1<<48)|(0x100<<32) {
		t.Errorf("idPrefix is %v", worker.idPrefix)
	}
	i := 0
	reqFunc := func(r *commandpb.Command) (*commandpb.CommandResult, error) {
		if r.Timestamp != (0x1<<48)|(0x100<<32)|uint64(i) {
			t.Error(r.Timestamp)
		}
		i++
		return nil, nil
	}
	worker.Init(reqFunc)
	worker.Run()
}
