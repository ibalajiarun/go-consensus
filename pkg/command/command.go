package command

import (
	"math/rand"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
)

// NewTestingCommand creates a writing *pb.Command with the provided start and
// end keys.
func NewTestingCommand(key string) *commandpb.Command {
	return &commandpb.Command{
		Timestamp: rand.Uint64(),
		Ops: []commandpb.Operation{
			{
				Type: &commandpb.Operation_KVOp{
					KVOp: &commandpb.KVOp{
						Key:   []byte(key),
						Value: []byte("random_value_for_testing"),
					},
				},
			},
		},
		ConflictKey: []byte(key),
	}
}
