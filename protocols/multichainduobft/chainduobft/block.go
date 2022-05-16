package chainduobft

import (
	"crypto/sha512"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

var genesis = &block{
	BlockState: pb.BlockState{
		View: 0,
		FastState: &pb.FastChainState{
			Command: nil,
			Height:  0,
			Parent:  nil,
			QC:      nil,
		},
		SlowState: &pb.SlowChainState{
			ProposeBlockHash: nil,
			Height:           0,
			Parent:           nil,
			QC:               nil,
		},
	},
}

type block struct {
	pb.BlockState
	blk512Hash *[sha512.Size]byte
}

func (b *block) hash() pb.BlockHash {
	if b.blk512Hash == nil {
		bBytes, err := proto.Marshal(&b.BlockState)
		if err != nil {
			panic(err)
		}
		shaHash := sha512.Sum512(bBytes)
		b.blk512Hash = new([sha512.Size]byte)
		*b.blk512Hash = shaHash
	}
	return *b.blk512Hash
}

func (b *block) String() string {
	return fmt.Sprintf("%v: %v", b.hash(), b.BlockState.String())
}

func newFastChainState(
	cmd *commandpb.Command,
	height pb.Height,
	parent pb.BlockHash,
	qc *pb.QuorumCert,
) *pb.FastChainState {
	return &pb.FastChainState{
		Command: cmd,
		Height:  height,
		Parent:  parent.Slice(),
		QC:      qc,
	}
}

func newSlowChainState(
	selfPropose bool,
	proposeBlockHeight pb.Height,
	proposeBlockHash pb.BlockHash,
	height pb.Height,
	parent pb.BlockHash,
	qc *pb.QuorumCert,
) *pb.SlowChainState {
	return &pb.SlowChainState{
		ProposeBlockHeight: proposeBlockHeight,
		ProposeBlockHash:   proposeBlockHash.Slice(),
		Height:             height,
		Parent:             parent.Slice(),
		QC:                 qc,
		SelfPropose:        selfPropose,
	}
}

func newBlock(
	view pb.View,
	fastState *pb.FastChainState,
	slowState *pb.SlowChainState,
) *block {
	b := &block{
		BlockState: pb.BlockState{
			View:      view,
			FastState: fastState,
			SlowState: slowState,
		},
	}
	return b
}
