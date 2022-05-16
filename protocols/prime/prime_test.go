package prime

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

func TestConfig(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	t.Logf("Current test filename: %s", filename)

	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			Workers: map[string]uint32{
				"mac_sign": 1,
			},
			WorkersQueueSizes: map[string]uint32{
				"mac_sign": 1,
			},
		},
		ID:     0,
		Peers:  []peerpb.PeerID{0, 1, 2},
		Logger: l,
	}
	p := NewPrime(c).(*prime)

	if p.id != c.ID {
		t.Errorf("expected Prime node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.nodes, c.Peers) {
		t.Errorf("expected Prime nodes %c, found %d", c.Peers, p.nodes)
	}
	if p.logger != l {
		t.Errorf("expected Prime logger %p, found %p", l, p.logger)
	}
	if p.f != (len(c.Peers)-1)/3 {
		t.Errorf("expected Prime node f %d, found %d", (len(c.Peers)-1)/2, p.f)
	}
}

func (s *prime) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *prime) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*prime
	failures    map[*prime]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount int) network {
	peers := make(map[peerpb.PeerID]*prime, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	f := (nodeCount - 1) / 3

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewPrime(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				Workers: map[string]uint32{
					"mac_sign": 1,
				},
				WorkersQueueSizes: map[string]uint32{
					"mac_sign": 1,
				},
				MaxFailures: int32(f),
			},
			ID:       r,
			Peers:    peersSlice,
			RandSeed: int64(r),
			Logger:   logger,
		}).(*prime)
	}
	return network{
		peers:    peers,
		failures: make(map[*prime]struct{}, nodeCount),
		dropm:    make(map[conn]float64),
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
	n.peers[id] = NewPrime(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*prime)
}

func (n *network) crash(id peerpb.PeerID) *prime {
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

func (n *network) alive(p *prime) bool {
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

func (n *network) tickAll() {
	time.Sleep(10 * time.Millisecond)
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

func (n *network) count(pred func(*prime) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*prime) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*prime) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *prime) bool, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}

	const maxTicks = 4000
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
	return n.runNetwork(func(p *prime) bool {
		return p.hasExecuted(inst.is.InstanceID)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4)

	n.peers[0].sendPrePrepare()
	n.peers[0].sendPrePrepare()
	n.peers[0].sendPrePrepare()
	n.peers[0].sendPrePrepare()

	i := 0
	n.runNetwork(func(p *prime) bool {
		i++
		if i > 5 {
			return true
		}
		return false
	}, false)

	if n.peers[0].prePrepareCounter > 0 {
		t.Fatal("Cannot send prepare without a request")
	}

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
	n := newNetwork(4)
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
	n := newNetwork(3)
	n.crashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestMultiExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(3)

	cmd1 := command.NewTestingCommand("a")
	cmd2 := command.NewTestingCommand("b")
	cmd3 := command.NewTestingCommand("e")

	inst1 := n.peers[0].onRequest(cmd1)
	inst2 := n.peers[1].onRequest(cmd2)
	inst3 := n.peers[2].onRequest(cmd3)

	if !n.waitExecuteInstance(inst1, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst1)
	}

	if !n.waitExecuteInstance(inst2, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst2)
	}

	if !n.waitExecuteInstance(inst3, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst3)
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	// increase poSummaryTimeout to 10 for this test to pass
	poSummaryTimeout = 20
	count := 19

	n := newNetwork(count)

	i := 0
	n.runNetwork(func(p *prime) bool {
		i++
		return i > 5
	}, false)

	if n.peers[0].prePrepareCounter > 0 {
		t.Fatal("Cannot send prepare without a request")
	}

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
