package chainhotstuffpb

import (
	"crypto/sha512"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

type View uint64

type BlockHash [sha512.Size]byte

type BlockHashSlice []byte

func (bh BlockHash) Slice() []byte {
	return bh[:]
}

func (bhs *BlockHashSlice) Array() [sha512.Size]byte {
	var hash [sha512.Size]byte
	copy(hash[:], *bhs)
	return hash
}

// WrapMessageInner wraps a union type of Message in a new isMessage_Type.
func WrapMessageInner(msg proto.Message) isChainHotstuffMessage_Type {
	switch t := msg.(type) {
	case *ProposeMessage:
		return &ChainHotstuffMessage_Propose{Propose: t}
	case *VoteMessage:
		return &ChainHotstuffMessage_Vote{Vote: t}
	case *NewViewMessage:
		return &ChainHotstuffMessage_NewView{NewView: t}
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in WrapMessageInner", t))
	}
}

// WrapHostuffMessage wraps a union type of Message in a new PBFTMessage without a
// destination.
func WrapHostuffMessage(msg proto.Message) *ChainHotstuffMessage {
	return &ChainHotstuffMessage{Type: WrapMessageInner(msg)}
}
