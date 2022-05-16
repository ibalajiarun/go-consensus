package mirbft

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
	p := NewMirBFT(c)

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

type mirNetwork struct {
	network.BaseNetwork
	mirPeers map[peerpb.PeerID]*MirBFT
}

func newNetwork(nodeCount int, failures int) *mirNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	mirPeers := make(map[peerpb.PeerID]*MirBFT, nodeCount)
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
		p := NewMirBFT(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], mirPeers[r] = p, p
	}
	return &mirNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, 5000),
		mirPeers:    mirPeers,
	}
}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *mirNetwork) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*MirBFT).hasExecuted(inst.is.Index)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1)

	cmd := command.NewTestingCommand("a")
	inst := n.mirPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, false /* quorum */) {
		t.Fatalf("command execution failed, inst %+v never installed", inst)
	}
}

func TestMultiExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1)

	cmd1 := command.NewTestingCommand("a")
	cmd2 := command.NewTestingCommand("b")
	cmd3 := command.NewTestingCommand("e")

	inst1 := n.mirPeers[0].onRequest(cmd1)
	inst2 := n.mirPeers[1].onRequest(cmd2)
	inst3 := n.mirPeers[2].onRequest(cmd3)

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

func TestExecuteCommandsOneNonSequenceFailure(t *testing.T) {
	n := newNetwork(4, 1)
	n.CrashN(n.F(), n.mirPeers[0].id)
	cmd := command.NewTestingCommand("a")
	inst := n.mirPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution failed, instance %+v never installed", inst)
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13
	failures := 6

	n := newNetwork(count, failures)
	instances := make([]*instance, count*5)
	for i := range instances {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		instances[i] = n.mirPeers[target].onRequest(cmd)
	}

	for _, inst := range instances {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}
}
