package chainhotstuff

import (
	"crypto/sha512"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

func (hs *chainhotstuff) startNewView() {
	hs.logger.Debug("starting NewView")
	msg := &pb.NewViewMessage{
		Height:  hs.bLeaf.Height,
		QC:      hs.highQC,
		Hash512: hs.bLeaf.hash().Slice(),
	}
	leaderID := hs.pacemaker.getLeader(hs.bLeaf.Height + 1)
	if leaderID == hs.id {
		// TODO: Is this necessary
		hs.onNewView(hs.id, msg)
		return
	}
	mBytes := hs.marshall(msg)
	hs.sendTo(leaderID, mBytes, nil)
}

func (hs *chainhotstuff) onNewView(from peerpb.PeerID, msg *pb.NewViewMessage) {
	defer func() {
		// cleanup
		for view := range hs.newView {
			if view < hs.bLeaf.Height {
				delete(hs.newView, view)
			}
		}
	}()

	hs.logger.Debug("OnNewView: ", msg)

	var hash [sha512.Size]byte
	copy(hash[:], msg.Hash512)

	hs.verifyDispatcher.Exec(func() []byte {
		if !hs.signer.verifyQuorumCert(msg.QC) {
			return []byte{0}
		}
		return []byte{1}
	}, func(res []byte) {
		if res[0] == 0 {
			hs.logger.Info("updateHighQC: QC could not be verified!")
		} else {
			hs.updateHighQC(msg.QC)
		}

		v, ok := hs.newView[msg.Height]
		if !ok {
			v = make(map[peerpb.PeerID]struct{})
		}
		v[from] = struct{}{}
		hs.newView[msg.Height] = v

		if len(v) < hs.signer.quorumSize {
			return
		}

		// signal the synchronizer
		hs.pacemaker.onNewView()
	})

}

func (hs *chainhotstuff) createDummy() {
	dummy := newBlock(hs.bLeaf.hash(), nil, nil, hs.bLeaf.Height+1, hs.id)
	hs.blocks[dummy.hash()] = dummy
	hs.bLeaf = dummy
}
