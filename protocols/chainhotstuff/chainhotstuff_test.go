package chainhotstuff

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
)

func TestConfig(t *testing.T) {
	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			MaxFailures:     1,
			MaxFastFailures: 0,
			Workers: map[string]uint32{
				"edd_verify": 1,
			},
			WorkersQueueSizes: map[string]uint32{
				"edd_verify": 8,
			},
		},
		ID:     3,
		Peers:  []peerpb.PeerID{0, 1, 2, 3},
		Logger: l,
	}
	p := NewChainHotstuff(c).(*chainhotstuff)

	if p.id != c.ID {
		t.Errorf("expected SDBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.peers, c.Peers) {
		t.Errorf("expected SDBFT nodes %c, found %d", c.Peers, p.peers)
	}
	if p.logger != l {
		t.Errorf("expected SDBFT logger %p, found %p", l, p.logger)
	}
	if p.f != (len(c.Peers)-1)/2 {
		t.Errorf("expected SDBFT node f %d, found %d", (len(c.Peers)-1)/2, p.f)
	}
}

func (s *chainhotstuff) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *chainhotstuff) ExecutableCommands() []peer.ExecPacket {
	cmds := s.committedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*chainhotstuff
	failures    map[*chainhotstuff]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount int, fastFailCount int32) network {
	peers := make(map[peerpb.PeerID]*chainhotstuff, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}

	// asyncSignerDoLock = true
	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewChainHotstuff(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				MaxFailures:     int32((nodeCount - int(2*fastFailCount)) / 3),
				MaxFastFailures: fastFailCount,
				DqOBatchSize:    10,
				Workers: map[string]uint32{
					"edd_verify": 1,
				},
				WorkersQueueSizes: map[string]uint32{
					"edd_verify": 8,
				},
			},
			ID:       r,
			Peers:    peersSlice,
			RandSeed: int64(r),
			Logger:   logger,
		}).(*chainhotstuff)
	}
	return network{
		peers:    peers,
		failures: make(map[*chainhotstuff]struct{}, nodeCount),
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
	n.peers[id] = NewChainHotstuff(&peer.LocalConfig{
		ID:       p.id,
		Peers:    p.peers,
		RandSeed: int64(id),
	}).(*chainhotstuff)
}

func (n *network) crash(id peerpb.PeerID) *chainhotstuff {
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

func (n *network) alive(p *chainhotstuff) bool {
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
	time.Sleep(1 * time.Millisecond)
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

func (n *network) count(pred func(*chainhotstuff) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*chainhotstuff) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*chainhotstuff) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *chainhotstuff) bool, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}

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
func (n *network) waitExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.runNetwork(func(p *chainhotstuff) bool {
		return p.hasExecuted(cmd)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 0)

	cmd := command.NewTestingCommand("a")
	n.peers[0].onRequest(cmd)

	if !n.waitExecuteInstance(cmd, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", cmd)
	}
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are F or fewer failures and the primary is active.
// func TestExecuteCommandsMinorityFailures(t *testing.T) {
// 	n := newNetwork(4, 0)
// 	n.crashN(n.F(), n.peers[1].id)
// 	cmd := command.NewTestingCommand("a")
// 	n.peers[1].onRequest(cmd)

// 	if !n.waitExecuteInstance(cmd, true /* quorum */) {
// 		t.Fatalf("command execution failed, instance %+v never installed", cmd)
// 	}
// }

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(4, 0)
	n.crashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	n.peers[0].onRequest(cmd)

	if n.waitExecuteInstance(cmd, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestManyCommandsNoFailuresSingleNode(t *testing.T) {
	paceMakerTimeout = 100
	count := 13

	n := newNetwork(count, 0)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmds[i] = cmd
		n.peers[0].onRequest(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			// for _, p := range n.peers {
			// 	t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			// }
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}

}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13

	n := newNetwork(count, 0)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmds[i] = cmd
		n.peers[peerpb.PeerID(i%count)].onRequest(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			// for _, p := range n.peers {
			// 	t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[cmd.is.Index], p.log[cmd.is.Index].is.Command)
			// }
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}

}
