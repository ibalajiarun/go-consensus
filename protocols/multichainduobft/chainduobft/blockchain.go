package chainduobft

import (
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

type blockchain struct {
	fastBlocks        map[pb.BlockHash]*block
	slowBlocks        map[pb.BlockHash]*block
	blockAtFastHeight map[pb.Height]*block
	blockAtSlowHeight map[pb.Height]*block
}

func newBlockchain() *blockchain {
	return &blockchain{
		fastBlocks:        make(map[pb.BlockHash]*block, 500),
		slowBlocks:        make(map[pb.BlockHash]*block, 500),
		blockAtFastHeight: make(map[pb.Height]*block, 500),
		blockAtSlowHeight: make(map[pb.Height]*block, 500),
	}
}

func (chain *blockchain) storeBlock(blk *block) {
	if blk.FastState != nil {
		chain.storeFastBlock(blk)
	}
	if blk.SlowState != nil {
		chain.storeSlowBlock(blk)
	}
}

func (chain *blockchain) storeFastBlock(blk *block) {
	chain.fastBlocks[blk.hash()] = blk
	chain.blockAtFastHeight[blk.FastState.Height] = blk
}

func (chain *blockchain) storeSlowBlock(blk *block) {
	chain.slowBlocks[blk.hash()] = blk
	chain.blockAtSlowHeight[blk.SlowState.Height] = blk
}

func (chain *blockchain) getBlockFor(hash pb.BlockHash) (*block, bool) {
	var blk *block
	var ok bool
	blk, ok = chain.fastBlocks[hash]
	if !ok {
		blk, ok = chain.slowBlocks[hash]
	}
	return blk, ok
}

func (chain *blockchain) getFastBlockFor(hash pb.BlockHash) (*block, bool) {
	blk, ok := chain.fastBlocks[hash]
	return blk, ok
}

func (chain *blockchain) getSlowBlockFor(hash pb.BlockHash) (*block, bool) {
	blk, ok := chain.slowBlocks[hash]
	return blk, ok
}

func (chain *blockchain) getAtFastHeight(height pb.Height) (*block, bool) {
	blk, ok := chain.blockAtFastHeight[height]
	return blk, ok
}

func (chain *blockchain) getAtSlowHeight(height pb.Height) (*block, bool) {
	blk, ok := chain.blockAtSlowHeight[height]
	return blk, ok
}

func (chain *blockchain) extendsFast(blk, target *block) bool {
	current := blk
	ok := true
	for ok && current.FastState.Height > target.FastState.Height {
		current, ok = chain.getFastBlockFor(current.FastState.Parent.Array())
	}
	return ok && current.hash() == target.hash()
}

func (chain *blockchain) extendsSlow(blk, target *block) bool {
	current := blk
	ok := true
	for ok && current.SlowState.Height > target.SlowState.Height {
		current, ok = chain.getSlowBlockFor(current.SlowState.Parent.Array())
	}
	return ok && current.hash() == target.hash()
}
