package chainduobftpb

import (
	"crypto/sha512"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

type Height uint64

type View uint64

type BlockHash [sha512.Size]byte

type BlockHashSlice []byte

func (bh BlockHash) Slice() []byte {
	return bh[:]
}

func (bh BlockHash) String() string {
	return fmt.Sprintf("%.4x", bh.Slice())
}

func (bhs BlockHashSlice) Array() [sha512.Size]byte {
	return *(*[sha512.Size]byte)(bhs)
}

func (sig *Signature) String() string {
	return fmt.Sprintf("<Counter: %d, Signer: %d>", sig.Counter, sig.Signer)
}

func (qc *QuorumCert) String() string {
	return fmt.Sprintf("BlockHash: %.4x, Height: %d, Signature: %v", qc.BlockHash, qc.Height, qc.Sigs)
}

func (fs *FastChainState) String() string {
	var cmd string
	if fs.Command != nil {
		cmd = string(fs.Command.ConflictKey)
	}
	return fmt.Sprintf("Height: %d, Parent: %.4x, QC: <<%v>>, Command: %v", fs.Height, fs.Parent, fs.QC, cmd)
}

func (ss *SlowChainState) String() string {
	return fmt.Sprintf("Height: %d, Parent: %.4x, QC: <<%v>>, ProposeBlockHash: %.4x, ProposeBlockHeight: %d, SelfPropose: %v", ss.Height, ss.Parent, ss.QC, ss.ProposeBlockHash, ss.ProposeBlockHeight, ss.SelfPropose)
}

func (bs *BlockState) String() string {
	return fmt.Sprintf("View: %d, FastState: <<%v>>, SlowState: <<%v>>", bs.View, bs.FastState, bs.SlowState)
}

func WrapMessageInner(msg proto.Message) isChainDuoBFTMessage_Type {
	switch t := msg.(type) {
	case *ProposeMessage:
		return &ChainDuoBFTMessage_Propose{Propose: t}
	case *VoteMessage:
		return &ChainDuoBFTMessage_Vote{Vote: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

func WrapChainDuoBFTMessage(msg proto.Message) *ChainDuoBFTMessage {
	return &ChainDuoBFTMessage{Type: WrapMessageInner(msg)}
}
