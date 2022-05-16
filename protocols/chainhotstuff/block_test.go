package chainhotstuff

import (
	"testing"

	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

func TestBlockHash(t *testing.T) {
	blocks := make(map[chainhotstuffpb.BlockHash]*block, 65536)
	b1 := &block{
		BlockState: chainhotstuffpb.BlockState{
			Parent:   genesis.hash().Slice(),
			Command:  &commandpb.Command{Timestamp: 1},
			Height:   1,
			QC:       nil,
			Proposer: 1,
		},
	}
	b2 := &block{
		BlockState: chainhotstuffpb.BlockState{
			Parent:   b1.hash().Slice(),
			Command:  &commandpb.Command{Timestamp: 2},
			Height:   2,
			QC:       nil,
			Proposer: 1,
		},
	}
	if b1.hash() == b2.hash() {
		t.Fatalf("Hashes are equal!!!")
	}
	b3 := &block{
		BlockState: b2.BlockState,
	}
	if b2.hash() != b3.hash() {
		t.Fatalf("Hashes are not equal!!!")
	}
	blocks[b1.hash()] = b1
	blocks[b2.hash()] = b2
	if len(blocks) != 2 {
		t.Fatalf("blocks size is not 2")
	}
}
