package duobft

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
	pb "github.com/ibalajiarun/go-consensus/protocols/duobft/duobftpb"
)

func TestConfig(t *testing.T) {
	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			SecretKeys:  []string{"thisisasamplekeyforenclave"},
			EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
			MaxFailures: 1,
			Workers: map[string]uint32{
				"mac_sign": 1,
			},
			WorkersQueueSizes: map[string]uint32{
				"mac_sign": 1,
			},
		},
		ID:     12,
		Peers:  []peerpb.PeerID{2, 5, 12},
		Logger: l,
	}
	p := NewDuoBFT(c).(*duobft)

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
		t.Errorf("expected PBFT node f %d, found %d", (len(c.Peers))/2, p.f)
	}
}

func (p *duobft) ReadMessages() []peerpb.Message {
	msgs := p.msgs
	p.ClearMsgs()
	return msgs
}

func (p *duobft) ExecutableCommands() []peer.ExecPacket {
	cmds := p.executedCmds
	p.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*duobft
	failures    map[*duobft]struct{}
	dropm       map[conn]float64
	delaym      map[conn]struct{}
	delayedm    map[conn][]peerpb.Message
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount, f int) network {
	peers := make(map[peerpb.PeerID]*duobft, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewDuoBFT(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				SecretKeys:  []string{"thisisasamplekeyforenclave"},
				EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
				MaxFailures: int32(f),
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
		}).(*duobft)
	}
	return network{
		peers:    peers,
		failures: make(map[*duobft]struct{}, nodeCount),
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
	n.peers[id] = NewDuoBFT(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*duobft)
}

func (n *network) crash(id peerpb.PeerID) *duobft {
	p := n.peers[id]
	n.failures[p] = struct{}{}
	return p
}

func (n *network) crashN(c int, except map[peerpb.PeerID]struct{}) {
	crashed := 0
	for r := range n.peers {
		if crashed >= c {
			return
		}
		if _, ok := except[r]; ok {
			continue
		}
		n.crash(r)
		crashed++
	}
}

func (n *network) alive(p *duobft) bool {
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

func (n *network) asyncCallbackAll() {
	for _, p := range n.peers {
		if n.alive(p) {
			p.AsyncCallback()
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

func (n *network) count(pred func(*duobft) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*duobft) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*duobft) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *duobft) bool, waitUntil func(func(*duobft) bool) bool) bool {
	// waitUntil := n.allHave
	// if quorum {
	// 	waitUntil = n.quorumHas
	// }
	const maxTicks = 10000
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

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitExecuteInstance(inst *instance, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}
	return n.runNetwork(func(p *duobft) bool {
		return p.hasExecuted(inst.is.Index)
	}, waitUntil)
}

func (n *network) waitTExecuteInstance(inst *instance, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}
	return n.runNetwork(func(p *duobft) bool {
		return p.hasTExecuted(inst.is.Index)
	}, waitUntil)
}

// waitCommitInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitCommitInstance(inst *instance, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}
	return n.runNetwork(func(p *duobft) bool {
		return p.hasCommitted(inst.is.Index)
	}, waitUntil)
}

// waitPrepareInstance waits until the given instance has prepared.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}
	return n.runNetwork(func(p *duobft) bool {
		return p.hasPrepared(inst.is.Index)
	}, waitUntil)
}

func (n *network) waitNewViewTransition(view pb.View, count int) bool {
	return n.runNetwork(func(p *duobft) bool {
		p.logger.Debugf("cur:%v stable:%v expected:%v", p.curView, p.stableView, view)
		return p.curView == p.stableView && p.stableView == view
	}, func(pred func(*duobft) bool) bool {
		return n.count(pred) == count
	})
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(7, 2)

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
	n := newNetwork(7, 2)
	n.crashN(n.F(), map[peerpb.PeerID]struct{}{n.peers[0].id: struct{}{}})
	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(7, 2)
	n.crashN(n.F()+1, nil)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestTExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(7, 2)

	cmd := command.NewTestingCommand("a")
	cmd.Meta = []byte{1}
	inst := n.peers[0].onRequest(cmd)

	if !n.waitTExecuteInstance(inst, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are F or fewer failures and the primary is active.
func TestTExecuteCommandsMinorityFailures(t *testing.T) {
	n := newNetwork(7, 2)
	n.crashN(n.F(), map[peerpb.PeerID]struct{}{n.peers[0].id: {}, n.peers[1].id: {}, n.peers[2].id: {}})
	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if !n.waitTExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestTExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(7, 2)
	n.crashN(5, nil)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if n.waitTExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13
	f := 4

	n := newNetwork(count, f)
	insts := make([]*instance, count*5)
	for i := range insts {
		cmd := command.NewTestingCommand("e")
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for i, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
		cmd := command.NewTestingCommand("e")
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

}

func TestTManyCommandsNoFailures(t *testing.T) {
	count := 13
	f := 4

	n := newNetwork(count, f)
	insts := make([]*instance, count*5)
	for i := range insts {
		cmd := command.NewTestingCommand("e")
		cmd.Meta = []byte{1}
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for i, inst := range insts {
		if !n.waitTExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
		cmd := command.NewTestingCommand("e")
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

	for i := range n.peers {
		if len(n.peers[i].executedCmds) != 65*2 {
			t.Fatalf("execCmds not 130: %v at replica %v", len(n.peers[i].executedCmds), i)
		}
	}

}
