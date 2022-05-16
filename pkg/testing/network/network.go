package network

import (
	"math/rand"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

type conn struct {
	from, to peerpb.PeerID
}

type Network interface {
}

type BaseNetwork struct {
	peers       map[peerpb.PeerID]peer.Protocol
	failedPeers map[peer.Protocol]struct{}
	dropm       map[conn]float64
	delaym      map[conn]struct{}
	delayedm    map[conn][]peerpb.Message
	interceptor func(peerpb.PeerID, peerpb.Message)
	failures    int
	runTicks    int
}

func NewNetwork(peers map[peerpb.PeerID]peer.Protocol, failures, runTicks int) BaseNetwork {
	return BaseNetwork{
		peers:       peers,
		failedPeers: make(map[peer.Protocol]struct{}, len(peers)),
		dropm:       make(map[conn]float64),
		delaym:      make(map[conn]struct{}),
		delayedm:    make(map[conn][]peerpb.Message),
		failures:    failures,
		runTicks:    runTicks,
	}
}

func (n *BaseNetwork) F() int {
	return n.failures
}

func (n *BaseNetwork) quorum(val int) bool {
	return val >= n.failures+1
}

func (n *BaseNetwork) setInterceptor(f func(from peerpb.PeerID, msg peerpb.Message)) {
	n.interceptor = f
}

func (n *BaseNetwork) Crash(id peerpb.PeerID) peer.Protocol {
	p := n.peers[id]
	n.failedPeers[p] = struct{}{}
	return p
}

func (n *BaseNetwork) CrashN(c int, except peerpb.PeerID) {
	crashed := 0
	for r := range n.peers {
		if crashed >= c {
			return
		}
		if r == except {
			continue
		}
		n.Crash(r)
		crashed++
	}
}

func (n *BaseNetwork) Alive(p peer.Protocol) bool {
	_, failed := n.failedPeers[p]
	return !failed
}

func (n *BaseNetwork) Delay(from, to peerpb.PeerID) {
	n.delaym[conn{from: from, to: to}] = struct{}{}
}

func (n *BaseNetwork) UnDelay(from, to peerpb.PeerID) {
	msgConn := conn{from: from, to: to}
	msgs := n.delayedm[msgConn]

	n.delayedm[msgConn] = nil
	delete(n.delaym, msgConn)

	for _, msg := range msgs {
		dest := n.peers[msg.To]
		if n.Alive(dest) {
			dest.Step(msg)
		}
	}
}

func (n *BaseNetwork) drop(from, to peerpb.PeerID, perc float64) {
	n.dropm[conn{from: from, to: to}] = perc
}

func (n *BaseNetwork) dropForAll(perc float64) {
	for from := range n.peers {
		for to := range n.peers {
			if from != to {
				n.drop(from, to, perc)
			}
		}
	}
}

func (n *BaseNetwork) cut(one, other peerpb.PeerID) {
	n.drop(one, other, 1.0)
	n.drop(other, one, 1.0)
}

func (n *BaseNetwork) isolate(id peerpb.PeerID) {
	for other := range n.peers {
		if other != id {
			n.cut(id, other)
		}
	}
}

func (n *BaseNetwork) asyncCallbackAll() {
	for _, p := range n.peers {
		if n.Alive(p) {
			p.AsyncCallback()
		}
	}
}

func (n *BaseNetwork) tickAll() {
	time.Sleep(5 * time.Millisecond)
	for _, p := range n.peers {
		if n.Alive(p) {
			p.Tick()
		}
	}
}

func (n *BaseNetwork) deliverAllMessages() {
	var msgs []peerpb.Message
	for r, p := range n.peers {
		if n.Alive(p) {
			rd := p.MakeReady()
			p.ClearMsgs()
			newMsgs := rd.Messages
			for _, msg := range newMsgs {
				if n.interceptor != nil {
					n.interceptor(r, msg)
				}
				msgConn := conn{from: msg.From, to: msg.To}
				perc := n.dropm[msgConn]
				if perc > 0 {
					if n := rand.Float64(); n < perc {
						continue
					}
				}
				if _, ok := n.delaym[msgConn]; ok {
					n.delayedm[msgConn] = append(n.delayedm[msgConn], msg)
					continue
				}
				msgs = append(msgs, msg)
			}
		}
	}
	for _, msg := range msgs {
		dest := n.peers[msg.To]
		if n.Alive(dest) {
			dest.Step(msg)
		}
	}
}

func (n *BaseNetwork) clearAllMessages() {
	for _, p := range n.peers {
		p.ClearMsgs()
	}
}

func (n *BaseNetwork) count(pred func(peer.Protocol) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *BaseNetwork) quorumHas(pred func(peer.Protocol) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *BaseNetwork) allHave(pred func(peer.Protocol) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *BaseNetwork) RunNetwork(goal func(p peer.Protocol) bool, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}

	for i := 0; i < n.runTicks; i++ {
		n.tickAll()
		n.deliverAllMessages()
		n.asyncCallbackAll()
		if waitUntil(goal) {
			return true
		}
	}
	return false
}
