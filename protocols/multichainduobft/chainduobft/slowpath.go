package chainduobft

import pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"

func (d *ChainDuoBFT) onSlowProposeContinue(m *pb.ProposeMessage, blk *block) {
	d.pacemaker.updateSlowHighQC(blk.SlowState.QC)

	qcBlock, haveQCBlock := d.blockchain.getSlowBlockFor(blk.SlowState.QC.BlockHash.Array())

	safe := false
	if haveQCBlock && qcBlock.SlowState.Height > d.bLockSlow.SlowState.Height {
		safe = true
	} else {
		d.logger.Debug("OnSlowPropose: liveness condition failed")
		// check if this block extends bLock
		if d.blockchain.extendsSlow(blk, d.bLockSlow) {
			safe = true
		} else {
			d.logger.Debug("OnSlowPropose: safety condition failed")
		}
	}

	if !safe {
		d.logger.Debug("OnSlowPropose: block not safe")
		d.queueBlockSlowPropose(m)
		return
	}

	// block is safe and was accepted
	d.blockchain.storeSlowBlock(blk)

	// we defer the following in order to speed up voting
	defer func() {
		if b := d.slowCommitRule(blk); b != nil {
			d.slowCommit(b)
		}
		d.dequeueSlowBlocks(blk)
		d.pacemaker.onSlowQC(blk.SlowState.QC)
	}()

	if blk.SlowState.Height <= d.lastVoteSlowHeight {
		d.logger.Info("OnSlowPropose: block height too old")
		return
	}

	if blk.FastState != nil {
		return
	}

	d.sendVote(blk)
}

func (d *ChainDuoBFT) slowCommitRule(blk *block) *block {
	block1, ok := d.qcRefSlow(blk.SlowState.QC)
	if !ok {
		return nil
	}

	if block1.SlowState.Height > d.bLockSlow.SlowState.Height {
		d.logger.Debug("LOCKSLOW: ", block1)
		d.bLockSlow = block1
	}

	block2, ok := d.qcRefSlow(block1.SlowState.QC)
	if !ok {
		return nil
	}

	if block1.SlowState.Parent.Array() == block2.hash() {
		d.logger.Debug("COMMITSLOW: ", block2)
		return block2
	}

	return nil
}

func (d *ChainDuoBFT) slowCommit(blk *block) {
	if d.bExecSlow.SlowState.Height < blk.SlowState.Height {
		if parent, ok := d.blockchain.getSlowBlockFor(blk.SlowState.Parent.Array()); ok {
			d.slowCommit(parent)
		} else {
			d.logger.Panicf("Refusing to commit because parent block could not be retrieved. %v.", blk)
			return
		}

		var execBlk *block
		if blk.SlowState.SelfPropose {
			execBlk = blk
		} else {
			var ok bool
			execBlk, ok = d.blockchain.getFastBlockFor(blk.SlowState.ProposeBlockHash.Array())
			if !ok {
				actBlk, _ := d.blockchain.getAtFastHeight(blk.SlowState.ProposeBlockHeight)
				d.logger.Debugf("Exec block is missing: blk: %v \n bLockFast: %v, \n actBlk: %v",
					blk, d.bLockFast, actBlk)
				return
			}
		}

		if execBlk.FastState == nil {
			d.logger.Fatalf("fast state is missing in slow exec block: %v\n actualBlk: %v", execBlk, blk)
		}

		d.logger.Debug("SLOWEXEC: ", execBlk, "\n actual Blk:", blk)
		d.bExecSlow = blk

		if execBlk.FastState.Command == nil || execBlk.FastState.Command.Meta != nil {
			// dont execute dummy blocks or fast path blocks
			return
		}
		d.enqueueForExecution(execBlk.FastState.Command)
		execBlk.FastState.Command = nil
	}
}

func (d *ChainDuoBFT) queueBlockSlowPropose(m *pb.ProposeMessage) {
	blk := &block{
		BlockState: m.BlockState,
	}
	qcBlkHash := blk.SlowState.QC.BlockHash.Array()
	d.logger.Debugf("Queueing slow proposal %.4x. waiting for %.4x",
		blk.hash(), qcBlkHash)
	if psp, ok := d.pendingSlowProposals[qcBlkHash]; ok {
		d.logger.Panicf("slow proposal is already pending for %.4x: %v", qcBlkHash, psp)
	}
	d.pendingSlowProposals[qcBlkHash] = m
}

func (d *ChainDuoBFT) dequeueSlowBlocks(blk *block) {
	bHash := blk.hash()
	if nextProposal, ok := d.pendingSlowProposals[bHash]; ok {
		d.logger.Debugf("dequeing slowqc for next block %.4x", bHash)
		delete(d.pendingSlowProposals, bHash)
		d.onSlowProposeContinue(nextProposal, &block{
			BlockState: nextProposal.BlockState,
		})
	}
}

func (d *ChainDuoBFT) qcRefSlow(qc *pb.QuorumCert) (*block, bool) {
	if qc == nil {
		return nil, false
	}
	return d.blockchain.getSlowBlockFor(qc.BlockHash.Array())
}
