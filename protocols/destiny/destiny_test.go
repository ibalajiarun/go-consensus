package destiny

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
)

var (
	keysMap = map[int]string{
		3: `3872398145671381750051310361161029866141599796961754550720954525373154029641
18913992266273626846214164704318441079079824927432633934738241159011855215319
7545865829532108925029707269152845926323190473050391644910514830901650171418
6044262478369222839265815623050805443715326867730809093545852876997568163314
13495860062249993865734902048793710375616417009831840885382880613253415974387
14432995725644194091092767544667816115262418605728523769974352756346168301805
15370131389038394316450633040541921854908420201625206654565824899438920629223`,
		13: `20996923786763358836803540769032373113757478809895585976194514339855933444842
66477225946307444284009547692497080210726363943710289170356375598226693126
8166779040760149919298452489980791807817387269120027653416053031190235658303
19012471463346271409320043689762312125909917726299258695246039394146625768671
3481193584861283656498579861920939692911393421262206495634791671905265117759
15243382379203800866914520369021252926615097504110042473604937139874892874133
7237090624768915640104942008878894538489716686243399549320406049138591144656
870999201708079981925076725096349032279238469657263534379266456107876901387
16641039382152395527521255167462464861349129762586990712621056433972692698532
16860227218642915881470739517169258735444609826021170355543922905631607410194
4140460852728844437964612438892769322888678601841659037005517675204711167112
12967366281662325014076853216650659285639527617598374622829200419219502544985
16256434455029100518366006818693840735755697868334884279013752824045576499998
9549364163386342321766314834519425175849357480457198622925054108487814998954
10191696256836347663811439340917696860947326352653359893091267692628378396608
15268224937214655538683591187567659758856576574695892824997947386069114551437
5849157611249681950073685683210258463227666225791255607675879841877619662314`,
	}
)

func readKeys(count int) ([]string, []string) {
	var skeys []string
	var vkeys []string

	keys, ok := keysMap[count]
	if !ok {
		log.Fatalf("key file not found: count=%d", count)
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
	skeys, vkeys := readKeys(3)
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			SecretKeys:       skeys,
			PublicKeys:       vkeys,
			EnclavePath:      "/home/balaji/workplace/mygo/go-consensus/enclaves/threshsign/enclave_threshsign/",
			EnclaveBatchSize: 1,
			MaxFailures:      1,
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
	p := NewDestiny(c).(*destiny)

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

func (s *destiny) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *destiny) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*destiny
	failures    map[*destiny]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount int) network {
	peers := make(map[peerpb.PeerID]*destiny, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(nodeCount)

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewDestiny(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				SecretKeys:       skeys,
				PublicKeys:       vkeys,
				EnclavePath:      "/home/balaji/workplace/mygo/go-consensus/enclaves/threshsign/enclave_threshsign/",
				EnclaveBatchSize: 1,
				MaxFailures:      int32(nodeCount-1) / 2,
				Workers: map[string]uint32{
					"tss_sign":   2,
					"tss_agg":    1,
					"tss_verify": 1,
				},
				WorkersQueueSizes: map[string]uint32{
					"tss_sign":   300,
					"tss_agg":    32,
					"tss_verify": 32,
				},
			},
			ID:       r,
			Peers:    peersSlice,
			RandSeed: int64(r),
			Logger:   logger,
		}).(*destiny)
	}
	return network{
		peers:    peers,
		failures: make(map[*destiny]struct{}, nodeCount),
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
	n.peers[id] = NewDestiny(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*destiny)
}

func (n *network) crash(id peerpb.PeerID) *destiny {
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

func (n *network) alive(p *destiny) bool {
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

func (n *network) count(pred func(*destiny) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*destiny) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*destiny) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *destiny) bool, quorum bool) bool {
	waitUntil := n.allHave
	if quorum {
		waitUntil = n.quorumHas
	}

	const maxTicks = 1000
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
	return n.runNetwork(func(p *destiny) bool {
		return p.hasExecuted(inst.is.InstanceID)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *destiny) bool {
		return p.hasPrepared(inst.is.InstanceID)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(3)

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
	n := newNetwork(3)
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
	count := 13

	n := newNetwork(count)
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
