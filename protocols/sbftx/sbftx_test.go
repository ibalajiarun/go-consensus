package sbftx

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
		"4_4": `10036052446616306226114094596945404101960435492575977716301194857204775076145
5518289815549875603598616958501098868810458719113235757804645761415932327121
12681278488763693231869849159509035064235382153197383251937034672238371437128
18557060260487301208324222725310509112715252975345611136986407692064969687024
14756620501597198494796990608793525338359814346116041458225065097271804861754
2569551292016190362508315314549980432571200999335596080692964548779431993676
12826866644963640281859987701291748015511451908792963750192053677326918084007
5904614305218804751007051663196764902306678861394133863403025344969074705247`,
		"5_6": `18480008935796592298615540406934461065248989045656446439663476714643172014464
13191385750097814186314680587825126578066562471652009156869590892619488916338
44859618481580811774415207139224813045432173289471462602500158422062066332
15476105971780499066894024748836305437917931575712428374802296901465521978978
11834380792246539710165111861211499013416268053159389379810073008399537452057
960633691752025037230990677911601964367071571279370830568448169229950209414
18342073071868192741504461454159076431448838335728503206092414548092171577440
4895585319214182255137110430997417499975244468818699673297271760469391602683
19252790119873137616320670393185566158792999475756934754677659858514148241146
9867498233697589881789104052390697495134753292025057823248400552428109223693`,
		"13_13": `20037943962760441401478019440824250045696493186208312621114541336441992159229
12017508488409304170589739863842407869679713832239793273360403535409077851038
4300003645864866090559097066614590676289853019894813307805950141892556608437
4931554312052468791415854460941491293631920319151428022179031555546003902930
12215138923291962595825225054188828311672286547516661745715131471178595114748
20981973552348555255941679243538459548697596337163686980529733019124583222955
7676497100603651627347381810489257951046780915336618783873213603578888426716
2734433731719849769725687560049412974767724621902303157387974777099006052538
9578128440266919025003007627990384307074082327891255937279910334581114321412
16807635276302576703591586923235282719386939402754300508876611504063824553646
8054944243054822163355468077427331671461999732144636910068787675856463145274
13109988213677378554397205830805184319984165886877956908767258030334269026745
6300465971691100020316274032131568721614037402857422512036658021163592187603
9298245404885592361052645519617356271770358341062554316994987408157066155628
10751706490030242830851874858526396826855511878827637333376596837682787242410
3262542288148279604472958080271052900851247594214188114359848930600016022704
10481719936901180747201111309824715868086578592016979895163385575009079894939`,
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
	p := NewSBFTx(c).(*sbftx)

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

func (s *sbftx) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *sbftx) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*sbftx
	failures    map[*sbftx]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount, fastFailures int) network {
	peers := make(map[peerpb.PeerID]*sbftx, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", nodeCount-fastFailures, nodeCount))

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewSBFTx(&peer.LocalConfig{
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
		}).(*sbftx)
	}
	return network{
		peers:    peers,
		failures: make(map[*sbftx]struct{}, nodeCount),
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
	n.peers[id] = NewSBFTx(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*sbftx)
}

func (n *network) crash(id peerpb.PeerID) *sbftx {
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

func (n *network) alive(p *sbftx) bool {
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

func (n *network) count(pred func(*sbftx) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*sbftx) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*sbftx) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *sbftx) bool, quorum bool) bool {
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
	return n.runNetwork(func(p *sbftx) bool {
		return p.hasExecuted(inst.is.InstanceID)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *sbftx) bool {
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
	n := newNetwork(6, 1)
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
