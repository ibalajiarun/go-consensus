package chainduobft

import (
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

func (d *ChainDuoBFT) onFastProposeContinue(m *pb.ProposeMessage, blk *block) {
	d.pacemaker.updateFastHighQC(blk.FastState.QC)

	qcBlock, haveQCBlock := d.blockchain.getFastBlockFor(blk.FastState.QC.BlockHash.Array())

	safe := false
	if haveQCBlock && qcBlock.FastState.Height > d.bLockFast.FastState.Height {
		safe = true
	} else {
		d.logger.Debug("OnFastPropose: liveness condition failed")
		// check if this block extends bLock
		if d.blockchain.extendsFast(blk, d.bLockFast) {
			safe = true
		} else {
			d.logger.Debug("OnFastPropose: safety condition failed")
		}
	}

	if !safe {
		d.logger.Debug("OnFastPropose: block not safe")
		d.queueBlockFastPropose(m)
		return
	}

	// block is safe and was accepted
	d.blockchain.storeFastBlock(blk)

	// we defer the following in order to speed up voting
	defer func() {
		if b := d.fastCommitRule(blk); b != nil {
			d.fastCommit(b)
		}
		d.dequeueFastBlocks(blk)
		d.pacemaker.onFastQC(blk.FastState.QC)
	}()

	if blk.FastState.Height <= d.lastVoteFastHeight {
		d.logger.Info("OnFastPropose: block height too old")
		return
	}

	d.sendVote(blk)
}

func (d *ChainDuoBFT) fastCommitRule(blk *block) *block {
	if blk.FastState.Height > d.bLockFast.FastState.Height {
		d.logger.Debug("LOCK: ", blk)
		d.bLockFast = blk
	}

	block1, ok := d.qcRefFast(blk.FastState.QC)
	if !ok {
		return nil
	}

	if blk.FastState.Parent.Array() == block1.hash() {
		d.logger.Debug("COMMIT: ", block1)
		return block1
	}

	return nil
}

func (d *ChainDuoBFT) fastCommit(blk *block) {
	if d.bExecFast.FastState.Height < blk.FastState.Height {
		if parent, ok := d.blockchain.getFastBlockFor(blk.FastState.Parent.Array()); ok {
			d.fastCommit(parent)
		} else {
			d.logger.Panicf("Refusing to fast commit because parent block could not be retrieved. \n Blk: %v, \n ExecBlk: %v \n LockBlk: %v \n", blk, d.bExecFast, d.bLockFast)
			return
		}

		d.logger.Debug("EXEC: ", blk)
		d.bExecFast = blk

		if blk.FastState.Command == nil || blk.FastState.Command.Meta == nil {
			// don't execute dummy blocks or slow path blocks
			return
		}
		d.enqueueForExecution(blk.FastState.Command)
		blk.FastState.Command = nil
	}
}

func (d *ChainDuoBFT) queueBlockFastPropose(m *pb.ProposeMessage) {
	blk := &block{
		BlockState: m.BlockState,
	}
	d.logger.Debugf("Queueing proposal %.4x. waiting for %.4x",
		blk.hash(), blk.FastState.QC.BlockHash.Array())
	d.pendingFastProposals[blk.FastState.QC.BlockHash.Array()] = m
}

func (d *ChainDuoBFT) dequeueFastBlocks(blk *block) {
	if nextProposal, ok := d.pendingFastProposals[blk.hash()]; ok {
		d.logger.Debugf("dequeing proposals for next block %.4x", blk.hash())
		delete(d.pendingFastProposals, blk.hash())
		d.onFastProposeContinue(nextProposal, &block{
			BlockState: nextProposal.BlockState,
		})
	}
}

func (d *ChainDuoBFT) qcRefFast(qc *pb.QuorumCert) (*block, bool) {
	if qc == nil {
		return nil, false
	}
	return d.blockchain.getFastBlockFor(qc.BlockHash.Array())
}

func (d *ChainDuoBFT) scheduleBackAfter(step func(), height pb.Height) {
	d.waitQueueForHeight[height] = step
}

func (d *ChainDuoBFT) runVoteForNextHeight(height pb.Height) {
	if step, ok := d.waitQueueForHeight[height]; ok {
		delete(d.waitQueueForHeight, height)
		step()
	}
}
