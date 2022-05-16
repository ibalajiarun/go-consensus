package hybsterx

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

func TestConfig(t *testing.T) {
	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			SecretKeys:      []string{"thisisasamplekeyforenclave"},
			EnclavePath:     "../../enclaves/trinx/enclave_trinx",
			MaxFailures:     1,
			MaxFastFailures: 0,
			Workers: map[string]uint32{
				"mac_sign": 2,
			},
			WorkersQueueSizes: map[string]uint32{
				"mac_sign": 1,
			},
		},
		ID:     12,
		Peers:  []peerpb.PeerID{2, 5, 12},
		Logger: l,
	}
	p := NewHybsterx(c).(*hybsterx)

	if p.id != c.ID {
		t.Errorf("expected PBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.nodes, c.Peers) {
		t.Errorf("expected PBFT nodes %c, found %d", c.Peers, p.nodes)
	}
	if p.logger != l {
		t.Errorf("expected PBFT logger %p, found %p", l, p.logger)
	}
	if p.f != len(c.Peers)/2 {
		t.Errorf("expected PBFT node f %d, found %d", (len(c.Peers)-1)/2, p.f)
	}
}

func (p *hybsterx) ReadMessages() []peerpb.Message {
	msgs := p.msgs
	p.ClearMsgs()
	return msgs
}

func (p *hybsterx) ExecutableCommands() []peer.ExecPacket {
	cmds := p.committedCommands
	p.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*hybsterx
	failures    map[*hybsterx]struct{}
	dropm       map[conn]float64
	delaym      map[conn]struct{}
	delayedm    map[conn][]peerpb.Message
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount, f int) network {
	peers := make(map[peerpb.PeerID]*hybsterx, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewHybsterx(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				SecretKeys:      []string{"thisisasamplekeyforenclave"},
				EnclavePath:     "../../enclaves/trinx/enclave_trinx/",
				MaxFailures:     int32(f),
				MaxFastFailures: 0,
				Workers: map[string]uint32{
					"mac_sign": 1,
				},
				WorkersQueueSizes: map[string]uint32{
					"mac_sign": 1,
				},
			},
			ID:       r,
			Peers:    peersSlice,
			RandSeed: int64(r),
			Logger:   logger,
		}).(*hybsterx)
	}
	return network{
		peers:    peers,
		failures: make(map[*hybsterx]struct{}, nodeCount),
		dropm:    make(map[conn]float64),
		delaym:   make(map[conn]struct{}),
		delayedm: make(map[conn][]peerpb.Message),
	}
}

func (n *network) F() int {
	return n.peers[0].f
}

func (n *network) quorum(val int) bool {
	return val >= n.peers[0].f+1
}

func (n *network) setInterceptor(f func(from peerpb.PeerID, msg peerpb.Message)) {
	n.interceptor = f
}

func (n *network) restart(id peerpb.PeerID) {
	p := n.peers[id]
	n.peers[id] = NewHybsterx(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*hybsterx)
}

func (n *network) crash(id peerpb.PeerID) *hybsterx {
	p := n.peers[id]
	n.failures[p] = struct{}{}
	return p
}

func (n *network) crashN(c int, except peerpb.PeerID) {
	crashed := 0
	for r := range n.peers {
		if crashed >= c {
			return
		}
		if r == except {
			continue
		}
		n.crash(r)
		crashed++
	}
}

func (n *network) alive(p *hybsterx) bool {
	_, failed := n.failures[p]
	return !failed
}

func (n *network) drop(from, to peerpb.PeerID, perc float64) {
	n.dropm[conn{from: from, to: to}] = perc
}

func (n *network) dropForAll(perc float64) {
	for from := range n.peers {
		for to := range n.peers {
			if from != to {
				n.drop(from, to, perc)
			}
		}
	}
}

func (n *network) cut(one, other peerpb.PeerID) {
	n.drop(one, other, 1.0)
	n.drop(other, one, 1.0)
}

func (n *network) isolate(id peerpb.PeerID) {
	for other := range n.peers {
		if other != id {
			n.cut(id, other)
		}
	}
}

func (n *network) delay(from, to peerpb.PeerID) {
	n.delaym[conn{from: from, to: to}] = struct{}{}
}

func (n *network) undelay(from, to peerpb.PeerID) {
	msgConn := conn{from: from, to: to}
	msgs := n.delayedm[msgConn]

	n.delayedm[msgConn] = nil
	delete(n.delaym, msgConn)

	for _, msg := range msgs {
		dest := n.peers[msg.To]
		if n.alive(dest) {
			dest.Step(msg)
		}
	}
}

func (n *network) asyncCallbackAll() {
	for _, p := range n.peers {
		if n.alive(p) {
			p.AsyncCallback()
		}
	}
}

func (n *network) tickAll() {
	time.Sleep(5 * time.Millisecond)
	for _, p := range n.peers {
		if n.alive(p) {
			p.Tick()
		}
	}
}

func (n *network) deliverAllMessages() {
	var msgs []peerpb.Message
	for r, p := range n.peers {
		if n.alive(p) {
			newMsgs := p.ReadMessages()
			for _, msg := range newMsgs {
				if n.interceptor != nil {
					n.interceptor(r, msg)
				}
				msgConn := conn{from: p.id, to: msg.To}
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
		if n.alive(dest) {
			dest.Step(msg)
		}
	}
}

func (n *network) clearAllMessages() {
	for _, p := range n.peers {
		p.ReadMessages()
	}
}

func (n *network) count(pred func(*hybsterx) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*hybsterx) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*hybsterx) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *hybsterx) bool, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}
	const maxTicks = 100
	for i := 0; i < maxTicks; i++ {
		n.tickAll()
		n.deliverAllMessages()
		n.asyncCallbackAll()
		if waitUntil(goal) {
			return true
		}
	}
	return false
}

// waitAcceptInstance waits until the given instance has Accepted.
// If quorum is true, it will wait until the instance is Accepted on
// a quorum of nodes. If it is true, it will wait until the instance
// is Accepted on all nodes.
//func (n *network) waitAcceptInstance(inst *instance, quorum bool) bool {
//	return n.runNetwork(func(p *hybsterx) bool {
//		return p.hasAccepted(inst.is.Index)
//	}, quorum)
//}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *hybsterx) bool {
		return p.hasExecuted(inst.is.InstanceID)
	}, quorum)
}

// waitPrepareInstance waits until the given instance has prepared.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *hybsterx) bool {
		return p.hasPrepared(inst.is.InstanceID)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(3, 1)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are F or fewer failures and the primary is active.
func TestExecuteCommandsMinorityFailures(t *testing.T) {
	n := newNetwork(3, 1)
	n.crashN(n.F(), n.peers[0].id)
	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(3, 1)
	n.crashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13

	n := newNetwork(count, (count-1)/2)
	insts := make([]*instance, count*5)
	for i := range insts {
		cmd := command.NewTestingCommand("e")
		insts[i] = n.peers[peerpb.PeerID(i%count)].onRequest(cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.InstanceID], p.log[inst.is.InstanceID].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

}
