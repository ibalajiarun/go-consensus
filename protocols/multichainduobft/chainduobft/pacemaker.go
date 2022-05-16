package chainduobft

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
)

type pacemaker struct {
	d *ChainDuoBFT

	bLeafFast         *block
	fastHighQC        *pb.QuorumCert
	fastHighQCChanged bool
	currentSlowHeight pb.Height

	bLeafSlow         *block
	slowHighQC        *pb.QuorumCert
	slowHighQCChanged bool
	currentFastHeight pb.Height

	currentView pb.View

	lastBeatFastHeight pb.Height
	lastBeatSlowHeight pb.Height
}

func newPacemaker(d *ChainDuoBFT, initialView pb.View) *pacemaker {
	highQC, err := d.signer.createQuorumCert(genesis.hash().Slice(), genesis.FastState.Height, []pb.VoteMessage{})
	if err != nil {
		d.logger.Panicf("Failed to create QC for genesis block!")
	}

	d.blockchain.storeBlock(genesis)

	return &pacemaker{
		d: d,

		bLeafFast:         genesis,
		fastHighQC:        highQC,
		fastHighQCChanged: true,
		currentFastHeight: 1,

		bLeafSlow:         genesis,
		slowHighQC:        highQC,
		slowHighQCChanged: true,
		currentSlowHeight: 1,

		currentView: initialView,

		lastBeatFastHeight: 0,
		lastBeatSlowHeight: 0,
	}
}

func (pm *pacemaker) beat() {
	if pm.d.id != pm.getLeader(pm.currentView) {
		return
	}

	if pm.currentFastHeight <= pm.lastBeatFastHeight && pm.currentSlowHeight <= pm.lastBeatSlowHeight {
		return
	}

	sent := pm.d.propose()
	if sent {
		if pm.currentFastHeight <= pm.lastBeatFastHeight {
			pm.lastBeatSlowHeight = pm.currentSlowHeight
		}
		if pm.currentSlowHeight <= pm.lastBeatSlowHeight {
			pm.lastBeatFastHeight = pm.currentFastHeight
		}
	}
}

func (pm *pacemaker) onRequest() {
	pm.beat()
}

func (pm *pacemaker) onFastQC(qc *pb.QuorumCert) {
	qcHeight := qc.Height

	if qcHeight < pm.currentFastHeight {
		return
	}

	pm.currentFastHeight = qcHeight + 1
	pm.d.scheduleBack(pm.beat)
}

func (pm *pacemaker) onSlowQC(qc *pb.QuorumCert) {
	qcHeight := qc.Height

	if qcHeight < pm.currentSlowHeight {
		return
	}

	pm.currentSlowHeight = qcHeight + 1
	pm.d.scheduleBack(pm.beat)
}

func (pm *pacemaker) updateFastHighQC(qc *pb.QuorumCert) {
	pm.d.logger.Debugf("updateFastHighQC: %.4x", qc.BlockHash)

	newBlock, ok := pm.d.blockchain.getFastBlockFor(qc.BlockHash.Array())
	if !ok {
		pm.d.logger.Debug("updateFastHighQC: Could not find block referenced by new QC!")
		return
	}

	oldBlock, ok := pm.d.blockchain.getFastBlockFor(pm.fastHighQC.BlockHash.Array())
	if !ok {
		pm.d.logger.Panic("Block from the old fastHighQC missing from chain")
	}

	if newBlock.FastState.Height > oldBlock.FastState.Height {
		pm.d.logger.Debug("FastHighQC updated")
		pm.fastHighQC = qc
		pm.bLeafFast = newBlock
		pm.fastHighQCChanged = true
	}
}

func (pm *pacemaker) updateSlowHighQC(qc *pb.QuorumCert) {
	pm.d.logger.Debugf("updateSlowHighQC: %.4x", qc.BlockHash)

	newBlock, ok := pm.d.blockchain.getSlowBlockFor(qc.BlockHash.Array())
	if !ok {
		pm.d.logger.Debug("updateSlowHighQC: Could not find block referenced by new QC!")
		return
	}

	oldBlock, ok := pm.d.blockchain.getSlowBlockFor(pm.slowHighQC.BlockHash.Array())
	if !ok {
		pm.d.logger.Panic("Block from the old slowHighQC missing from chain")
	}

	if newBlock.SlowState.Height > oldBlock.SlowState.Height {
		pm.d.logger.Debug("SlowHighQC updated")
		pm.slowHighQC = qc
		pm.bLeafSlow = newBlock
		pm.slowHighQCChanged = true
	}
}

func (pm *pacemaker) getLeader(view pb.View) peerpb.PeerID {
	numPeers := len(pm.d.peers)
	return peerpb.PeerID(view % pb.View(numPeers))
}
