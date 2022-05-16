package chainduobft

import (
	"encoding/binary"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

func (d *ChainDuoBFT) propose() bool {
	var fastState *pb.FastChainState
	var slowState *pb.SlowChainState

	if d.pacemaker.fastHighQCChanged {
		var cmd *commandpb.Command
		cmdElem := d.cmdQueue.Front()
		if cmdElem != nil {
			d.cmdQueue.Remove(cmdElem)
			cmd = cmdElem.Value.(*commandpb.Command)
			d.lastFastHeightWithCommand = d.pacemaker.currentFastHeight
		}

		if d.lastFastHeightWithCommand+1 >= d.lastFastHeight {
			d.lastFastHeight = d.pacemaker.currentFastHeight
			fastState = newFastChainState(
				cmd,
				d.pacemaker.currentFastHeight,
				d.pacemaker.bLeafFast.hash(),
				d.pacemaker.fastHighQC,
			)
			d.pacemaker.fastHighQCChanged = false
		}
	}

	if d.pacemaker.slowHighQCChanged {
		var nextBlkHeight pb.Height
		var nextBlkHash pb.BlockHash
		var selfPropose bool

		if fastState != nil && d.pacemaker.currentFastHeight == d.pacemaker.currentSlowHeight {
			selfPropose = true
			d.lastSlowHeightWithCommand = d.pacemaker.currentSlowHeight
		} else {
			var ok bool
			nextBlk, ok := d.blockchain.getAtFastHeight(d.pacemaker.currentSlowHeight)
			if !ok {
				goto sendBlock
			}
			nextBlkHash = nextBlk.hash()
			nextBlkHeight = nextBlk.FastState.Height
			d.lastSlowHeightWithCommand = d.pacemaker.currentSlowHeight
		}

		if d.lastSlowHeightWithCommand+1 >= d.lastSlowHeight {
			d.lastSlowHeight = d.pacemaker.currentSlowHeight
			slowState = newSlowChainState(
				selfPropose,
				nextBlkHeight,
				nextBlkHash,
				d.pacemaker.currentSlowHeight,
				d.pacemaker.bLeafSlow.hash(),
				d.pacemaker.slowHighQC,
			)
			d.pacemaker.slowHighQCChanged = false
		}
	}

sendBlock:
	if fastState == nil && slowState == nil {
		return false
	}

	blk := newBlock(d.pacemaker.currentView, fastState, slowState)

	m := &pb.ProposeMessage{
		BlockState: blk.BlockState,
	}

	d.blockchain.storeBlock(blk)

	mBytes := d.marshall(m)
	d.broadcast(mBytes, false, nil)

	nextStep := func() {
		if blk.FastState != nil {
			d.onFastProposeContinue(m, blk)
		}

		if blk.SlowState != nil {
			d.onSlowProposeContinue(m, blk)
		}
	}

	d.scheduleBack(nextStep)

	return true
}

func (d *ChainDuoBFT) onPropose(m *pb.ProposeMessage, from peerpb.PeerID) {
	blk := &block{
		BlockState: m.BlockState,
	}

	// ensure the block came from the leader and matches currentView.
	if from != d.pacemaker.getLeader(blk.View) && blk.View == d.pacemaker.currentView {
		d.logger.Info("OnPropose: block was not proposed by the expected leader")
		return
	}

	d.logger.Debugf("OnPropose: %v", blk)

	if blk.FastState != nil {
		d.fastVerifyDispatcher.Exec(func() []byte {
			if !d.signer.verifyQuorumCert(blk.FastState.QC, d.fastQuorumSize) {
				return []byte{0}
			}
			return []byte{1}
		}, func(res []byte) {
			if res[0] == 0 {
				d.logger.Fatalf("onPropose: FastQC could not be verified! %.4x, %v, %v, %v",
					blk.hash(),
					blk.FastState.Height, len(blk.FastState.QC.Sigs), blk.FastState.QC.Sigs[0])
			} else {
				d.onFastProposeContinue(m, blk)
			}
		})
	}

	if blk.SlowState != nil {
		d.slowVerifyDispatcher.Exec(func() []byte {
			if !d.signer.verifyQuorumCert(blk.SlowState.QC, d.slowQuorumSize) {
				return []byte{0}
			}
			return []byte{1}
		}, func(res []byte) {
			if res[0] == 0 {
				d.logger.Fatalf("onPropose: SlowQC could not be verified! %.4x, %v, %v, %v",
					blk.hash(),
					blk.SlowState.Height, len(blk.SlowState.QC.Sigs), blk.SlowState.QC.Sigs[0])
			} else {
				d.onSlowProposeContinue(m, blk)
			}
		})
	}
}

func (d *ChainDuoBFT) sendVote(blk *block) {
	if blk.FastState != nil && blk.FastState.Height != d.lastVoteFastHeight+1 {
		d.scheduleBackAfter(func() {
			d.sendVote(blk)
		}, blk.FastState.Height-1)
		return
	}

	signWithUsig := blk.FastState != nil

	signStep := func() []byte {
		var ctrsig []byte
		var err error
		if signWithUsig {
			ctrsig, err = d.signer.signUsig(blk.hash())
		} else {
			ctrsig, err = d.signer.sign(blk.hash())
		}
		if err != nil {
			d.logger.Error("OnPropose: failed to sign vote: ", err)
			return nil
		}
		return ctrsig
	}

	postSignStep := func(ctrsig []byte) {
		if blk.FastState != nil {
			d.lastVoteFastHeight = blk.FastState.Height
			d.runVoteForNextHeight(d.lastVoteFastHeight)
		}

		if blk.SlowState != nil {
			d.lastVoteSlowHeight = blk.SlowState.Height
		}

		ctr := binary.LittleEndian.Uint64(ctrsig[:8])
		sig := ctrsig[8:]

		m := &pb.VoteMessage{
			BlockHash: blk.hash().Slice(),
			Signature: &pb.Signature{
				Sig:     sig,
				Signer:  d.id,
				Counter: ctr,
			},
		}

		leaderID := d.pacemaker.getLeader(d.pacemaker.currentView)
		if leaderID == d.id {
			d.onVote(m)
			return
		}
		mBytes := d.marshall(m)
		d.sendTo(leaderID, mBytes, nil)
	}

	if signWithUsig {
		d.signUsigDispatcher.Exec(signStep, postSignStep)
	} else {
		d.signDispatcher.Exec(signStep, postSignStep)
	}
}

func (d *ChainDuoBFT) onVote(vote *pb.VoteMessage) {
	d.logger.Debugf("OnVote(%d) %.4x: %v", vote.Signature.Signer, vote.BlockHash, vote)

	blk, ok := d.blockchain.getBlockFor(vote.BlockHash.Array())
	if !ok {
		// d.logger.Info(d.blockchain.fastBlocks)
		// d.logger.Info(d.blockchain.slowBlocks)
		d.logger.Error("Could not find block for vote: %.4x: %v", vote.BlockHash, vote)
		return
	}

	if blk.FastState != nil && blk.FastState.Height <= d.pacemaker.bLeafFast.FastState.Height && blk.SlowState == nil {
		// too old
		return
	}

	if blk.SlowState != nil && blk.SlowState.Height <= d.pacemaker.bLeafSlow.SlowState.Height {
		// too old
		return
	}

	d.fastVerifyDispatcher.Exec(func() []byte {
		if !d.signer.verify(blk.hash(), vote.Signature) {
			d.logger.Debug("OnVote: Vote could not be verified!")
			return []byte{0}
		}
		return []byte{1}
	}, func(res []byte) {
		if res[0] == 0 {
			d.logger.Fatal("onVote: Vote could not be verified!")
		} else {
			d.onVoteContinue(vote, blk)
		}
	})
}

func (d *ChainDuoBFT) onVoteContinue(vote *pb.VoteMessage, blk *block) {
	d.logger.Debugf("onVoteContinue(%d): %.4x", vote.Signature.Signer, vote.BlockHash)
	// this defer will clean up any old votes in verifiedVotes
	defer func() {
		// delete any pending QCs with lower height than bLeaf
		for k := range d.verifiedVotes {
			if block, ok := d.blockchain.getBlockFor(k); ok {
				if block.SlowState != nil && block.SlowState.Height <= d.pacemaker.bLeafSlow.SlowState.Height {
					delete(d.verifiedVotes, k)
				}
				if block.SlowState == nil && block.FastState != nil && block.FastState.Height <= d.pacemaker.bLeafFast.FastState.Height {
					delete(d.verifiedVotes, k)
				}
			} else {
				delete(d.verifiedVotes, k)
			}
		}
	}()

	votes := d.verifiedVotes[vote.BlockHash.Array()]
	votes = append(votes, *vote)
	d.verifiedVotes[vote.BlockHash.Array()] = votes

	if blk.FastState != nil {
		if len(votes) < d.fastQuorumSize {
			return
		}

		if len(votes) == d.fastQuorumSize {
			qc, err := d.signer.createQuorumCert(blk.hash().Slice(), blk.FastState.Height, votes)
			if err != nil {
				d.logger.Info("OnVote: could not create QC for block: ", err)
				return
			}

			d.pacemaker.updateFastHighQC(qc)
			d.pacemaker.onFastQC(qc)
		}
	}

	if blk.SlowState == nil || d.skipSlowPath {
		return
	}

	if len(votes) < d.slowQuorumSize {
		return
	}

	qc, err := d.signer.createQuorumCert(blk.hash().Slice(), blk.SlowState.Height, votes)
	if err != nil {
		d.logger.Info("OnVote: could not create QC for block: ", err)
		return
	}

	delete(d.verifiedVotes, vote.BlockHash.Array())

	d.pacemaker.updateSlowHighQC(qc)
	d.pacemaker.onSlowQC(qc)
}
