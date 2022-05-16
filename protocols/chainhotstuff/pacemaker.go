package chainhotstuff

import (
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
)

var (
	paceMakerTimeout = 18000
)

type pacemaker struct {
	hs       *chainhotstuff
	lastBeat pb.View
	started  bool
	timer    peer.TickingTimer
}

func newPacemaker(hs *chainhotstuff) *pacemaker {
	pm := &pacemaker{
		hs: hs,
	}
	pm.timer = peer.MakeTickingTimer(paceMakerTimeout, pm.onTimeout)
	// hs.registerInfiniteTimer(pm.timer)
	return pm
}

func (pm *pacemaker) onPropose() {
	pm.timer.Reset()
}

func (pm *pacemaker) onFinishQC() {
	pm.beat()
}

func (pm *pacemaker) onNewView() {
	pm.beat()
}

func (pm *pacemaker) beat() {
	view := pm.hs.bLeaf.Height

	if view <= pm.lastBeat {
		return
	}

	if pm.hs.id != pm.getLeader(view+1) {
		return
	}
	pm.lastBeat = view

	pm.hs.propose()
}

func (pm *pacemaker) getLeader(view pb.View) peerpb.PeerID {
	return peerpb.PeerID(view % pb.View(len(pm.hs.peers)))
}

func (pm *pacemaker) onTimeout() {
	pm.hs.createDummy()
	pm.hs.startNewView()
	pm.timer.Reset()
}
