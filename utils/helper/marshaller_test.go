package helper

import (
	"math/rand"
	"testing"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/utils/signer"
)

func BenchmarkMarshallAndSign(b *testing.B) {
	msg := &commandpb.CommandResult{
		Timestamp: rand.Uint64(),
		OpResults: []commandpb.OperationResult{
			{
				Type: &commandpb.OperationResult_KVOpResult{
					KVOpResult: &commandpb.KVOpResult{
						Key:   []byte("hello-world-123456789"),
						Value: nil,
					},
				},
			},
		},
	}
	signer := signer.NewSigner()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			MarshallAndSign(msg, signer)
		}
	})
}
