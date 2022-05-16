package dqsbftslow

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	pb "github.com/ibalajiarun/go-consensus/protocols/sbftx/sbftxpb"
)

var (
	keysMap = map[string]string{
		"4_4": `18742699708598496074664972082124928190264525562889593911874577182468775273120
12818539260555989020933383692525940577176622836777898968633375145706318539529
19161217533723428947849252169456919686089979634039367545665433683298544298770
5881076760192488196999786279680554939007455672976610242630819138911684798215
3537726319553018215337395269647490413136013399074109126767192890611084672050
4320453458096852045076390255020465685134991448043976504899489486853032810860
9131989909260869837201569154984854607601529891258677945149785779939256003795
17069838973309844458375154297426947268544904439830305917339888491894116194802`,
		"13_13": `16117857080775839223754954331721961331073679986276688627046645506133931093005
1785122917015298126080507303953791094844394428550915500621627012402141740451
11736694234375060581403030616080562567445790894968824766696440179508332977163
15965665831818954759525135490696492324838810868470134312443159098726914144743
1522077530327456337541632390669881330235020390889730091868683190395314735194
5359791180407585293577508923489849726130732900909979787138002426810371454312
4197024878197116673725133799634124051549376830615120661596342569978273317727
21619579050497648323142950050368983934979055780726960265855775264461885695396
12246103796233124078558799988048248067204032668765639818432441935222853821265
3024310188656119685569812551797138125187514788371095797634061496362629671722
13628532331599004270516374104935572982515477553673721618763610412164290308607
6460981001050261156905757534409786019679819553688022331156731576553263073549
17081171153585477167269939416243257822448285024604217946513064007809024373946
21460787134740914413187724933337152562894054598053847498819300134169195631400
7533630668813580067683390693781989824305381435189293778927700359529972552336
3182215464659166695938585612137765192813443938386269721727406402950959952969
17619872687170953986851059569255789357374611256362911331573149360867055155506`,
	}
)

func readKeys(count string) ([]string, []string) {
	var skeys []string
	var vkeys []string

	keys, ok := keysMap[count]
	if !ok {
		log.Fatalf("key file not found: count=%s", count)
	}

	scanner := bufio.NewScanner(strings.NewReader(keys))
	idx := 0
	for idx < 4 && scanner.Scan() {
		vkey := scanner.Text()
		vkeys = append(vkeys, vkey)
		idx++
	}

	if idx < 4 {
		log.Fatal("too few lines in file")
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	for scanner.Scan() {
		skey := scanner.Text()
		skeys = append(skeys, skey)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return skeys, vkeys
}

func TestConfig(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	t.Logf("Current test filename: %s", filename)

	l := logger.NewDefaultLogger()
	skeys, vkeys := readKeys("4_4")
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			SecretKeys:       skeys,
			PublicKeys:       vkeys,
			EnclavePath:      "/home/balaji/workplace/mygo/go-consensus/enclaves/threshsign2/enclave_threshsign/",
			EnclaveBatchSize: 1,
			MaxFailures:      1,
			MaxFastFailures:  0,
			Workers: map[string]uint32{
				"tss_sign":   1,
				"tss_agg":    1,
				"tss_verify": 1,
			},
			WorkersQueueSizes: map[string]uint32{
				"tss_sign":   8,
				"tss_agg":    8,
				"tss_verify": 8,
			},
		},
		ID:     0,
		Peers:  []peerpb.PeerID{0, 1, 2},
		Logger: l,
	}
	p := NewDQSBFTSlow(c).(*dqsbftslow)

	if p.id != c.ID {
		t.Errorf("expected SDBFT node ID %d, found %d", c.ID, p.id)
	}
	if !reflect.DeepEqual(p.nodes, c.Peers) {
		t.Errorf("expected SDBFT nodes %c, found %d", c.Peers, p.nodes)
	}
	if p.logger != l {
		t.Errorf("expected SDBFT logger %p, found %p", l, p.logger)
	}
	if p.f != (len(c.Peers)-1)/2 {
		t.Errorf("expected SDBFT node f %d, found %d", (len(c.Peers)-1)/2, p.f)
	}
}

func (s *dqsbftslow) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *dqsbftslow) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*dqsbftslow
	failures    map[*dqsbftslow]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount, fastFailures int) network {
	peers := make(map[peerpb.PeerID]*dqsbftslow, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", nodeCount-fastFailures, nodeCount))

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewDQSBFTSlow(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				SecretKeys:       skeys,
				PublicKeys:       vkeys,
				EnclavePath:      "/home/balaji/workplace/mygo/go-consensus/enclaves/threshsign2/enclave_threshsign/",
				EnclaveBatchSize: 1,
				MaxFailures:      int32((nodeCount - int(2*fastFailures)) / 3),
				MaxFastFailures:  int32(fastFailures),
				Workers: map[string]uint32{
					"tss_sign":   1,
					"tss_agg":    1,
					"tss_verify": 1,
				},
				WorkersQueueSizes: map[string]uint32{
					"tss_sign":   8,
					"tss_agg":    8,
					"tss_verify": 8,
				},
			},
			ID:       r,
			Peers:    peersSlice,
			RandSeed: int64(r),
			Logger:   logger,
		}).(*dqsbftslow)
	}
	return network{
		peers:    peers,
		failures: make(map[*dqsbftslow]struct{}, nodeCount),
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
	n.peers[id] = NewDQSBFTSlow(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*dqsbftslow)
}

func (n *network) crash(id peerpb.PeerID) *dqsbftslow {
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

func (n *network) alive(p *dqsbftslow) bool {
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

func (n *network) count(pred func(*dqsbftslow) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*dqsbftslow) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*dqsbftslow) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *dqsbftslow) bool, quorum bool) bool {
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
func (n *network) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *dqsbftslow) bool {
		return p.hasExecuted(inst.is.InstanceID)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *dqsbftslow) bool {
		return p.hasPrepared(inst.is.InstanceID)
	}, quorum)
}

func TestThresholdSignature(t *testing.T) {
	n := newNetwork(4, 0)

	cm := &pb.NormalMessage{
		View:        0,
		InstanceID:  pb.InstanceID{ReplicaID: 2, Index: 3},
		Type:        pb.NormalMessage_SignShare,
		CommandHash: []byte("987654321"),
	}
	mBytes := n.peers[0].marshall(cm)

	sigs := make(map[peerpb.PeerID][]byte)
	for _, p := range n.peers {
		sigs[p.id] = p.signer.Sign(mBytes, uint64(100))
		t.Logf("sig len %d", len(sigs[p.id]))
	}
	combinedSig := n.peers[0].signer.AggregateSigAndVerify(mBytes, uint64(100), sigs)
	n.peers[0].signer.Verify(mBytes, uint64(100), combinedSig)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 0)

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
	n := newNetwork(4, 0)
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
	n := newNetwork(4, 0)
	n.crashN(n.F()+1, 0)

	cmd := command.NewTestingCommand("a")
	inst := n.peers[0].onRequest(cmd)

	if n.waitExecuteInstance(inst, true /* quorum */) {
		t.Fatalf("command execution succeeded with minority of nodes")
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13

	n := newNetwork(count, 0)
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
