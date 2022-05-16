package pbft

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/testing/network"
)

func TestConfig(t *testing.T) {
	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			MaxFailures:     1,
			MaxFastFailures: 0,
			Workers: map[string]uint32{
				"mac_sign": 1,
			},
			WorkersQueueSizes: map[string]uint32{
				"mac_sign": 1,
			},
		},
		ID:     12,
		Peers:  []peerpb.PeerID{2, 5, 12, 77},
		Logger: l,
	}
	p := NewPBFT(c)

	if p.id != c.ID {
		t.Errorf("expected PBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.nodes, c.Peers) {
		t.Errorf("expected PBFT nodes %c, found %d", c.Peers, p.nodes)
	}
	if p.logger != l {
		t.Errorf("expected PBFT logger %p, found %p", l, p.logger)
	}
	if p.f != (len(c.Peers)-1)/2 {
		t.Errorf("expected PBFT node f %d, found %d", (len(c.Peers)-1)/2, p.f)
	}
}

type pbftNetwork struct {
	network.BaseNetwork
	pbftPeers map[peerpb.PeerID]*PBFT
}

func newNetwork(nodeCount int, failures int) *pbftNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	mirPeers := make(map[peerpb.PeerID]*PBFT, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}

	peerConfig := &peerpb.PeerConfig{
		MaxFailures: int32(failures),
		Workers: map[string]uint32{
			"mac_sign": 1,
		},
		WorkersQueueSizes: map[string]uint32{
			"mac_sign": 1,
		},
	}

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Peer%d ", r))
		logger.EnableDebug()
		p := NewPBFT(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], mirPeers[r] = p, p
	}
	return &pbftNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, 2000),
		pbftPeers:   mirPeers,
	}
}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *pbftNetwork) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*PBFT).hasExecuted(inst.is.Index)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *pbftNetwork) waitPrePrepareInstance(inst *instance, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*PBFT).hasPrePrepared(inst.is.Index)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1)

	cmd := command.NewTestingCommand("a")
	inst := n.pbftPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are F or fewer failures and the primary is active.
func TestExecuteCommandsMinorityFailures(t *testing.T) {
	n := newNetwork(4, 1)
	n.CrashN(n.F(), n.pbftPeers[0].id)
	cmd := command.NewTestingCommand("a")
	inst := n.pbftPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(4, 1)
	n.CrashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	inst := n.pbftPeers[0].onRequest(cmd)

	if n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestExecuteCommandsAfterViewChange(t *testing.T) {
	n := newNetwork(4, 1)

	cmd := command.NewTestingCommand("a")
	inst := n.pbftPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, false /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}

	n.Crash(0)

	count := 0
	for _, p := range n.pbftPeers {
		if n.Alive(p) {
			if count >= 2*n.F()+1 {
				break
			}
			p.startViewChange(p.view + 1)
			count++
		}
	}

	if !n.waitPrePrepareInstance(inst, true /* quorum */) {
		t.Fatalf("command prepreparation failed, instance %+v never installed", inst)
	}

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		for _, p := range n.pbftPeers {
			t.Logf("instance %+v", p.log[inst.is.Index])
		}
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}

}
