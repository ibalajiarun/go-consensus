package dispel

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
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
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
	p := NewDispel(c)

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
		t.Errorf("expected PBFT node f %d, found %d", (len(c.Peers)-1)/3, p.maxFailures)
	}
}

type dispelNetwork struct {
	network.BaseNetwork
	dispelPeers map[peerpb.PeerID]*Dispel
}

func newNetwork(nodeCount int, failures int) *dispelNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	mirPeers := make(map[peerpb.PeerID]*Dispel, nodeCount)
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
		p := NewDispel(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], mirPeers[r] = p, p
	}
	return &dispelNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, 5000),
		dispelPeers: mirPeers,
	}
}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *dispelNetwork) waitExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*Dispel).hasExecuted(cmd)
	}, quorum)
}

// waitBroadcastInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *dispelNetwork) waitBroadcastInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*Dispel).hasBroadcast(cmd)
	}, quorum)
}

func (n *dispelNetwork) waitExecuteEpoch(eNum pb.Epoch, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*Dispel).epochs[eNum].consDoneCount == len(n.dispelPeers)
	}, quorum)
}

func TestNoFailuresAllProposeFastConsensusAfterAllRB(t *testing.T) {
	count := 4
	failures := 1

	n := newNetwork(count, failures)
	for _, p := range n.dispelPeers {
		p.rbThreshold = len(n.dispelPeers)
	}

	commands := make([]*commandpb.Command, count)
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for _, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed, cmd %+v never broadcasted", cmd)
		}
	}

	for _, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, cmd %+v never installed", cmd)
		}
	}
}

func TestNoFailuresAllProposeFastConsensusAfterMajorityRB(t *testing.T) {
	count := 4
	failures := 1

	n := newNetwork(count, failures)

	commands := make([]*commandpb.Command, count)
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for _, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed, cmd %+v never broadcasted", cmd)
		}
	}

	for _, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, cmd %+v never installed", cmd)
		}
	}
}

func TestCommandsNoFailuresAllPropose(t *testing.T) {
	count := 4
	failures := 1
	epochNumber := pb.Epoch(0)

	n := newNetwork(count, failures)
	for _, p := range n.dispelPeers {
		p.rbThreshold = len(n.dispelPeers)
		ep := &epoch{
			epochNum:    epochNumber,
			rbDoneCount: len(n.dispelPeers),

			rbs:  make(map[peerpb.PeerID]*rbroadcast),
			cons: make(map[peerpb.PeerID]*dbft),
		}
		p.epochs[epochNumber] = ep
		p.tryStartConsensus(ep)
	}

	if !n.waitExecuteEpoch(epochNumber, false /* quorum */) {
		t.Fatalf("command execution failed, epoch %+v never installed", epochNumber)
	}
}

func TestCommandsNoFailuresMajorityPropose(t *testing.T) {
	count := 4
	failures := 1

	n := newNetwork(count, failures)
	commands := make([]*commandpb.Command, count-failures)
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for _, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed, cmd %+v never broadcasted", cmd)
		}
	}

	for _, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, cmd %+v never installed", cmd)
		}
	}
}

func TestManyCommandsNoFailuresAllProposeMajorityConsensus(t *testing.T) {
	count := 13
	failures := 4

	n := newNetwork(count, failures)
	commands := make([]*commandpb.Command, count)
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for i, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed: %d, cmd %+v never broadcasted", i, cmd)
		}
	}

	for i, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed: %d, cmd %+v never installed", i, cmd)
		}
	}
}

func TestManyCommandsNoFailuresMajorityProposeMajorityConsensus(t *testing.T) {
	count := 13
	failures := 4

	n := newNetwork(count, failures)
	commands := make([]*commandpb.Command, 5*(count-failures))
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % (count - failures))
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for i, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed: %d, cmd %+v never broadcasted", i, cmd)
		}
	}

	for i, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed: %d, cmd %+v never installed", i, cmd)
		}
	}
}

func TestManyCommandsMinorityFailuresAllProposeMajorityConsensus(t *testing.T) {
	count := 9
	failures := 4

	n := newNetwork(count, failures)
	commands := make([]*commandpb.Command, 4*count)
	for i := range commands {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		commands[i] = cmd
		n.dispelPeers[target].onRequest(cmd)
	}

	for i, cmd := range commands {
		if !n.waitBroadcastInstance(cmd, false /* quorum */) {
			t.Fatalf("command broadcast failed: %d, cmd %+v never broadcasted", i, cmd)
		}
	}

	for i, cmd := range commands {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed: %d, cmd %+v never installed", i, cmd)
		}
	}
}
