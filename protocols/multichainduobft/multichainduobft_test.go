package multichainduobft

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/testing/network"
)

var runTicks int = 1000

func TestConfig(t *testing.T) {
	l := logger.NewDefaultLogger()
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			MaxFailures: 1,
			Workers: map[string]uint32{
				"mcd_edd_usig_sign": 2,
				"mcd_edd_sign":      2,
				"mcd_edd_verify":    2,
			},
			WorkersQueueSizes: map[string]uint32{
				"mcd_edd_usig_sign": 32,
				"mcd_edd_sign":      32,
				"mcd_edd_verify":    32,
			},
			EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
			MultiChainDuoBFT: &peerpb.MultiChainDuoBFTConfig{
				RCCMode:          false,
				InstancesPerPeer: 2,
			},
		},
		ID:     12,
		Peers:  []peerpb.PeerID{2, 5, 12, 77},
		Logger: l,
	}
	p := NewMultiChainDuoBFT(c)

	if p.id != c.ID {
		t.Errorf("expected DuoBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.peers, c.Peers) {
		t.Errorf("expected DuoBFT nodes %c, found %d", c.Peers, p.peers)
	}
	if p.logger != l {
		t.Errorf("expected DuoBFT logger %p, found %p", l, p.logger)
	}

}

type multichainduobftNetwork struct {
	network.BaseNetwork
	mcdPeers map[peerpb.PeerID]*multichainduobft
}

func newNetwork(nodeCount, failures, instances int, rcc bool) *multichainduobftNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	mcdPeers := make(map[peerpb.PeerID]*multichainduobft, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}

	peerConfig := &peerpb.PeerConfig{
		MaxFailures: int32(failures),
		Workers: map[string]uint32{
			"mcd_edd_usig_sign": 2,
			"mcd_edd_sign":      2,
			"mcd_edd_verify":    2,
		},
		WorkersQueueSizes: map[string]uint32{
			"mcd_edd_usig_sign": 32,
			"mcd_edd_sign":      32,
			"mcd_edd_verify":    32,
		},
		EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
		MultiChainDuoBFT: &peerpb.MultiChainDuoBFTConfig{
			RCCMode:          rcc,
			InstancesPerPeer: uint32(instances),
		},
	}

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Peer%d ", r))
		logger.EnableDebug()
		p := NewMultiChainDuoBFT(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], mcdPeers[r] = p, p
	}
	return &multichainduobftNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, runTicks),
		mcdPeers:    mcdPeers,
	}
}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *multichainduobftNetwork) waitExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*multichainduobft).hasExecuted(cmd)
	}, quorum)
}

func (n *multichainduobftNetwork) waitFastExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*multichainduobft).hasFastExecuted(cmd)
	}, quorum)
}

func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1, 1, false)

	cmd := command.NewTestingCommand("a")
	n.mcdPeers[0].Request(cmd)

	if !n.waitExecuteInstance(cmd, false /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v never committed", cmd)
	}
}

func TestExecuteCommandsMinorityFailures(t *testing.T) {
	n := newNetwork(4, 1, 1, false)
	n.CrashN(n.F(), n.mcdPeers[0].id)
	cmd := command.NewTestingCommand("a")
	n.mcdPeers[0].Request(cmd)

	if !n.waitExecuteInstance(cmd, true /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v never committed", cmd)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(3, 1, 1, false)
	n.CrashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	n.mcdPeers[0].Request(cmd)

	if n.waitExecuteInstance(cmd, true /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v committed", cmd)
	}
}

func TestManyCommandsNoFailuresFast(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 1, false)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmd.Meta = []byte{1}
		cmds[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresSlow(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 1, false)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmds[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresFastMultiInstances(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 2, false)
	cmds := make([]*commandpb.Command, count*5*10)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmd.Meta = []byte{1}
		cmds[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, true /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}

	cmds2 := make([]*commandpb.Command, count*5*10)
	for i := range cmds2 {
		cmd := command.NewTestingCommand("e")
		cmd.Meta = []byte{1}
		cmds2[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}

	for _, cmd := range cmds2 {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresSlowMultiInstances(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 5, false)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand(fmt.Sprintf("e-%d", i))
		cmds[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for i := peerpb.PeerID(maxFailures + 1); i < peerpb.PeerID(count); i++ {
		n.Delay(i, 0)
	}

	for _, cmd := range cmds {
		if !n.waitFastExecuteInstance(cmd, true /* quorum */) {
			for pid, p := range n.mcdPeers {
				if !p.hasFastExecuted(cmd) {
					t.Logf("cmd not executed in peer %v", pid)
				}
			}
			t.Fatalf("1: command execution failed, instance %+v never installed", cmd)
		}
	}

	cmds2 := make([]*commandpb.Command, count*5)
	for i := range cmds2 {
		cmd := command.NewTestingCommand(fmt.Sprintf("e2-%d", i))
		cmds2[i] = cmd
		n.mcdPeers[0].Request(cmd)
	}

	for i := peerpb.PeerID(maxFailures + 1); i < peerpb.PeerID(count); i++ {
		n.UnDelay(i, 0)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("2: command execution failed, instance %+v never installed", cmd)
		}
	}

	for _, cmd := range cmds2 {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("3: command execution failed, instance %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresFastMultiInstancesRCC(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 1, true)
	cmds := make([]*commandpb.Command, count*5*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		cmd.Meta = []byte{1}

		target := peerpb.PeerID(i % count)
		cmd.Target = target

		cmds[i] = cmd
		n.mcdPeers[target].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresSlowMultiInstancesRCC(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures, 5, true)
	cmds := make([]*commandpb.Command, count*5*5)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target

		cmds[i] = cmd
		n.mcdPeers[target].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}

}
