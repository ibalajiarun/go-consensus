package chainduobft

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
				"edd_sign":   16,
				"edd_verify": 16,
			},
			WorkersQueueSizes: map[string]uint32{
				"edd_sign":   128,
				"edd_verify": 128,
			},
			EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
		},
		ID:     12,
		Peers:  []peerpb.PeerID{2, 5, 12, 77},
		Logger: l,
	}
	p := NewChainDuoBFT(c)

	if p.id != c.ID {
		t.Errorf("expected PBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.peers, c.Peers) {
		t.Errorf("expected PBFT nodes %c, found %d", c.Peers, p.peers)
	}
	if p.logger != l {
		t.Errorf("expected PBFT logger %p, found %p", l, p.logger)
	}
	if p.maxFailures != (len(c.Peers)-1)/2 {
		t.Errorf("expected PBFT node f %d, found %d", (len(c.Peers)-1)/2, p.maxFailures)
	}
}

type duobftNetwork struct {
	network.BaseNetwork
	duobftPeers map[peerpb.PeerID]*ChainDuoBFT
}

func newNetwork(nodeCount int, failures int) *duobftNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	duobftPeers := make(map[peerpb.PeerID]*ChainDuoBFT, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}

	peerConfig := &peerpb.PeerConfig{
		MaxFailures: int32(failures),
		Workers: map[string]uint32{
			"edd_sign":   16,
			"edd_verify": 16,
		},
		WorkersQueueSizes: map[string]uint32{
			"edd_sign":   128,
			"edd_verify": 128,
		},
		EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/",
	}

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Peer%d ", r))
		logger.EnableDebug()
		p := NewChainDuoBFT(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], duobftPeers[r] = p, p
	}
	return &duobftNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, runTicks),
		duobftPeers: duobftPeers,
	}
}

func (n *duobftNetwork) waitExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*ChainDuoBFT).hasExecuted(cmd)
	}, quorum)
}

func (n *duobftNetwork) waitFastExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*ChainDuoBFT).HasFastExecuted(cmd)
	}, quorum)
}

func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1)

	cmd := command.NewTestingCommand("a")
	n.duobftPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(cmd, false /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v never committed", cmd)
	}
}

func TestExecuteCommandsMinorityFailures(t *testing.T) {
	n := newNetwork(4, 1)
	n.CrashN(n.F(), n.duobftPeers[0].id)
	cmd := command.NewTestingCommand("a")
	n.duobftPeers[0].onRequest(cmd)

	if !n.waitExecuteInstance(cmd, true /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v never committed", cmd)
	}
}

// TestExecuteCommandsNoFailures verifies that no replica can make forward
// progress whether there are more than F failures, excluding primary.
func TestExecuteCommandsMajorityFailures(t *testing.T) {
	n := newNetwork(3, 1)
	n.CrashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	n.duobftPeers[0].onRequest(cmd)

	if n.waitExecuteInstance(cmd, true /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v committed", cmd)
	}
}

func TestManyCommandsNoFailuresFast(t *testing.T) {
	count := 13
	maxFailures := 4

	n := newNetwork(count, maxFailures)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand(fmt.Sprintf("e-%d", i))
		cmd.Meta = []byte{1}
		cmds[i] = cmd
		n.duobftPeers[0].onRequest(cmd)
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

	n := newNetwork(count, maxFailures)
	cmds := make([]*commandpb.Command, count*5)
	for i := range cmds {
		cmd := command.NewTestingCommand(fmt.Sprintf("e-%d", i))
		cmds[i] = cmd
		n.duobftPeers[0].onRequest(cmd)
	}

	for i := peerpb.PeerID(maxFailures + 1); i < peerpb.PeerID(count); i++ {
		n.Delay(0, i)
		n.Delay(i, 0)
	}

	for _, cmd := range cmds {
		if !n.waitFastExecuteInstance(cmd, true /* quorum */) {
			for pid, p := range n.duobftPeers {
				if !p.HasFastExecuted(cmd) {
					t.Logf("cmd did not fast execute at peer %d", pid)
				}
			}
			t.Fatalf("1: command execution failed, instance %+v never installed", cmd)
		}
	}

	cmds2 := make([]*commandpb.Command, count*5)
	for i := range cmds2 {
		cmd := command.NewTestingCommand(fmt.Sprintf("e2-%d", i))
		cmds2[i] = cmd
		n.duobftPeers[0].Request(cmd)
	}

	for i := peerpb.PeerID(maxFailures + 1); i < peerpb.PeerID(count); i++ {
		n.UnDelay(0, i)
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
