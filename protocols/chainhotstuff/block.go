package chainhotstuff

import (
	"crypto/sha512"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

var genesis = &block{
	BlockState: pb.BlockState{
		Parent:   nil,
		Command:  nil,
		Height:   0,
		QC:       nil,
		Proposer: 0,
	},
}

type block struct {
	pb.BlockState
	blk512Hash *[sha512.Size]byte
}

func newBlock(
	parent pb.BlockHash, cmd *commandpb.Command,
	qc *pb.QuorumCert, height pb.View, proposer peerpb.PeerID,
) *block {
	return &block{
		BlockState: pb.BlockState{
			Parent:   parent[:],
			Command:  cmd,
			Height:   height,
			QC:       qc,
			Proposer: proposer,
		},
	}
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

func (hs *chainhotstuff) propose() {
	var cmd *commandpb.Command
	cmdElem := hs.cmdQueue.Front()
	if cmdElem != nil {
		hs.cmdQueue.Remove(cmdElem)
		cmd = cmdElem.Value.(*commandpb.Command)
	} else {
		cmd = nil
	}

	blk := newBlock(hs.bLeaf.hash(), cmd, hs.highQC, hs.bLeaf.Height+1, hs.id)
	blkHash := blk.hash()
	hs.blocks[blkHash] = blk
	hs.logger.Debugf("Blocks length %v", len(hs.blocks))

	m := &pb.ProposeMessage{
		BlockState: blk.BlockState,
	}
	mBytes := hs.marshall(m)
	hs.broadcast(mBytes, false, nil)
	hs.onPropose(m)
}

func (hs *chainhotstuff) onPropose(m *pb.ProposeMessage) {
	blk := &block{
		BlockState: m.BlockState,
	}
	hs.logger.Debugf("Replica %d ====[Propose,%.4x,(%v,%v)]====>>> Replica %d\n", m.Proposer, blk.hash(), m.BlockState.Height, m.BlockState.Command, hs.id)

	if blk.Proposer != hs.pacemaker.getLeader(blk.Height) {
		hs.logger.Panic("Very bad")
		return
	}

	if blk.Height <= hs.lastVote {
		hs.logger.Info("OnPropose: block view was less than our view")
		return
	}

	hs.verifyDispatcher.Exec(func() []byte {
		if !hs.signer.verifyQuorumCert(blk.QC) {
			return []byte{0}
		}
		return []byte{1}
	}, func(res []byte) {
		if res[0] == 0 {
			hs.logger.Fatal("updateHighQC: QC could not be verified! ", blk.Height, len(blk.QC.Sigs))
		} else {
			hs.onProposeContinue(m, blk)
		}
	})
}

func (hs *chainhotstuff) onProposeContinue(m *pb.ProposeMessage, blk *block) {
	qcBlock, haveQCBlock := hs.blocks[blk.QC.BlockHash.Array()]

	safe := false
	if haveQCBlock && qcBlock.Height > hs.bLock.Height {
		safe = true
	} else {
		hs.logger.Debug("OnPropose: liveness condition failed")
		// check if this block extends bLock
		b := blk
		ok := true
		for ok && b.Height > hs.bLock.Height {
			b, ok = hs.blocks[b.Parent.Array()]
		}
		if ok && b.hash() == hs.bLock.hash() {
			safe = true
		} else {
			hs.logger.Debug("OnPropose: safety condition failed")
		}
	}

	if !safe {
		hs.logger.Debug("OnPropose: block not safe")
		hs.queueBlockPropose(m)
		return
	}

	// Signal the synchronizer
	hs.pacemaker.onPropose()

	sig, err := hs.signer.sign(blk.hash())
	if err != nil {
		hs.logger.Error("OnPropose: failed to sign vote: ", err)
		return
	}

	hs.blocks[blk.hash()] = blk
	hs.lastVote = blk.Height

	finish := func() {
		hs.update(blk)
		hs.deliver(blk)
	}

	vm := &pb.VoteMessage{
		BlockHash: blk.hash().Slice(),
		Signature: &pb.Signature{
			Sig:    sig,
			Signer: hs.id,
		},
	}

	leaderID := hs.pacemaker.getLeader(hs.lastVote + 1)
	if leaderID == hs.id {
		finish()
		hs.onVote(vm)
		return
	}

	vmBytes := hs.marshall(vm)
	hs.sendTo(leaderID, vmBytes, nil)
	finish()
}

func (hs *chainhotstuff) onVote(m *pb.VoteMessage) {
	hs.logger.Debugf("Replica %d ====[Vote,%.4x]====>>> Replica %d\n", m.Signature.Signer, m.BlockHash, hs.id)

	defer func() {
		// delete any pending QCs with lower height than bLeaf
		for k := range hs.verifiedVotes {
			if block, ok := hs.blocks[k]; ok {
				if block.Height <= hs.bLeaf.Height {
					delete(hs.verifiedVotes, k)
				}
			} else {
				delete(hs.verifiedVotes, k)
			}
		}
	}()

	blk, ok := hs.blocks[m.BlockHash.Array()]
	if !ok {
		hs.logger.Debugf("Could not find block for vote: %x. queueing.", m.BlockHash)
		hs.queueBlockVote(m)
		return
	}

	if blk.Height <= hs.bLeaf.Height {
		// too old
		return
	}

	hs.verifyDispatcher.Exec(func() []byte {
		if !hs.signer.verify(blk.hash(), m.Signature.Sig) {
			hs.logger.Debug("OnVote: Vote could not be verified!")
			return []byte{0}
		}
		return []byte{1}
	}, func(res []byte) {
		if res[0] == 0 {
			hs.logger.Fatal("updateHighQC: QC could not be verified!")
		} else {
			hs.onVoteContinue(m, blk)
		}
	})
}

func (hs *chainhotstuff) onVoteContinue(m *pb.VoteMessage, blk *block) {
	hs.logger.Debugf("OnVote from %v: %.4x", m.Signature.Signer, m.BlockHash)

	votes := hs.verifiedVotes[blk.hash()]
	votes = append(votes, *m)
	hs.verifiedVotes[blk.hash()] = votes

	if len(votes) < 2*hs.f+1 {
		return
	}

	qc, err := hs.signer.createQuorumCert(blk, votes)
	if err != nil {
		hs.logger.Fatalf("OnVote: could not create QC for block: ", err)
	}
	delete(hs.verifiedVotes, blk.hash())

	hs.updateHighQC(qc)
	// signal the synchronizer
	hs.pacemaker.onFinishQC()
}

func (hs *chainhotstuff) updateHighQC(qc *pb.QuorumCert) {
	hs.logger.Debugf("updateHighQC: %v", qc)

	newBlock, ok := hs.blocks[qc.BlockHash.Array()]
	if !ok {
		hs.logger.Debug("updateHighQC: Could not find block referenced by new QC!")
		return
	}

	oldBlock, ok := hs.blocks[hs.highQC.BlockHash.Array()]
	if !ok {
		hs.logger.Panic("Block from the old highQC missing from chain")
	}

	if newBlock.Height > oldBlock.Height {
		hs.highQC = qc
		hs.bLeaf = newBlock
	}
}

func (hs *chainhotstuff) update(blk *block) {
	block1, ok := hs.qcRef(blk.QC)
	if !ok {
		return
	}

	hs.logger.Debug("PRE_COMMIT: ", block1.Height)

	hs.updateHighQC(blk.QC)

	block2, ok := hs.qcRef(block1.QC)
	if !ok {
		return
	}

	if block2.Height > hs.bLock.Height {
		hs.logger.Debug("COMMIT: ", block2.Height)
		hs.bLock = block2
	}

	block3, ok := hs.qcRef(block2.QC)
	if !ok {
		return
	}

	if block1.Parent.Array() == block2.hash() && block2.Parent.Array() == block3.hash() {
		hs.logger.Debug("DECIDE: ", block3.Height)
		hs.commit(block3)
		hs.bExec = block3
	}

}

func (hs *chainhotstuff) deliver(blk *block) {
	if votes, ok := hs.pendingVotes[blk.hash()]; ok {
		hs.logger.Debugf("OnDeliver: %v", blk.hash())

		delete(hs.pendingVotes, blk.hash())

		hs.blocks[blk.hash()] = blk

		for _, vote := range votes {
			vote := vote
			hs.onVote(&vote)
		}
	}

	if nextProposal, ok := hs.pendingProposals[blk.hash()]; ok {
		hs.logger.Debugf("dequeing proposals for next block %.4x", blk.hash())
		delete(hs.pendingProposals, blk.hash())
		hs.onPropose(nextProposal)
	}
}

func (hs *chainhotstuff) qcRef(qc *pb.QuorumCert) (*block, bool) {
	if qc == nil {
		return nil, false
	}
	blk, ok := hs.blocks[qc.BlockHash.Array()]
	return blk, ok
}

func (hs *chainhotstuff) commit(blk *block) {
	if hs.bExec.Height < blk.Height {
		if parent, ok := hs.blocks[blk.Parent.Array()]; ok {
			hs.commit(parent)
		}
		if blk.QC == nil || blk.Command == nil {
			// don't execute dummy nodes
			return
		}
		hs.logger.Debug("EXEC: ", blk.hash())
		hs.execute(blk.Command)
	}
}

func (hs *chainhotstuff) queueBlockVote(vote *pb.VoteMessage) {
	hash := vote.BlockHash.Array()
	votes := hs.pendingVotes[hash]
	votes = append(votes, *vote)
	hs.pendingVotes[hash] = votes
}

func (hs *chainhotstuff) queueBlockPropose(m *pb.ProposeMessage) {
	blk := &block{
		BlockState: m.BlockState,
	}
	hs.logger.Debugf("Queueing proposal %.4x. waiting for %.4x", blk.hash(), blk.QC.BlockHash.Array())
	hs.pendingProposals[blk.QC.BlockHash.Array()] = m
}
