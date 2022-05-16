package linhybster

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
		3: `2332931861564678021114989114841857189682043206391682009459806404313982065046
16583649684013498120883435974379211908371332517272047853942348768636327725122
11138386070939536688617948208402473165401162256308618419870066907689595654176
6320902695478525154682193517943914023716632562438641956118414933399601675836
15528999124991923814275242148138035147059301161054036428873714485750328622430
18593557609962679954989843948248064364468956751100957934153760667190418387865
4538714113992300144654627795175237443785252615171820718836567365858307226511`,
		13: `11308493038721233835629199775690672535650191038009428578865860230940090464766
4456850602782079208304846391051642729410198554494421432900939720626963461424
16948702935841286981891270930883416415154657637771667674284127586279294368231
7284607790652927514349048008696084527961305650849999232797651291227236930583
1644571674876591099812830404988905289142464504976163944023352112914812424264
19546069369865666720973488935641384011134678519473995506625507722101233091285
5050577253776459340565324469107945398630326804556235275686134319030801200041
12029026712799507514617808440455511712913053955210336461463646294512292713432
2102839535665434361975640881419614181941210099947856094421684729518680901763
17673701828377131093106780977027867843390895040981484808943719610159373802571
10453918647331860766067572498193027642662598955075171674642435827360069389066
3943533572436515014494875265561739540335655150088875230326326832020160912157
5773878756341896147871768204311955722419930055069540975738465700888945830855
8500514544904788399769400277825772014017571412698564805090842022605781490107
11705298136348408070578086743556494444758495983525687057562704905425373555384
8074067203553022486669768741891597270664432488277593207681137151444065258305
8453219425484699285675545401046434750490654133825403254345753583617621639740`,
		19: `10078865111067708438789574755919029279338674261094991806704123257918260493603
7893236644700225498680506111948135188882677500285720367655638028897317905603
9286406198754148052812795897243454729740358841942155364185197441430548604261
18759182179271563439905007321294703628573615274797119449813478825599449540729
3890077781157513884080651959974679780857632988276742796261526647302684293461
3480292474991046375826019317788266160217627285425712174422682639530348983568
10861310691988885450499644816948096373888639826526601681801796140798662981131
2224028191616106967729889458663349244851864130141095181498792011797780039852
18503526584799595301139043654629811416753334554108163182102540415856226232988
10686369361967959172375360728732706600546029311265583455338107497177626222681
11880676195281148562382041841802309171912493649545589821535944785944878784680
18300481731373214207854538677375123866193992983558608602754047598944012335726
10933746539328728810075933357686897094692032829286847811768455986799621950421
11293712892681923305570021000128433385696268818594482484024341977094762080718
3040661397880657164906524620547726905444007031381003671335619099238299173189
10104669853992805713070369302660933558289893790787284987558879033734269045354
647131155879326566575021288956219282166537481584826786742212554681747744816
16039784529213068143370398076527086868967728989519251306246075357798315585405
16922965558253363884963299523881997355413117518624621257469293759025788897487
10661063665395358422843242321817883489988313274217866031696612187559062907423
4396394398824621807049082340542996089590488571790833884052717541516255085932
7052972741136341291053820672634979117282934071686823885319114995871290638972
15867396433473267199256560768379943756621831310124370185096074424836018122892`,
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
			EnclavePath:      "../../enclaves/threshsign2/enclave_threshsign/",
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
	p := NewLinHybster(c).(*linhybster)

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

func (s *linhybster) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *linhybster) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*linhybster
	failures    map[*linhybster]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount int) network {
	peers := make(map[peerpb.PeerID]*linhybster, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(nodeCount)

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewLinHybster(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				SecretKeys:       skeys,
				PublicKeys:       vkeys,
				EnclavePath:      "../../enclaves/threshsign2/enclave_threshsign/",
				EnclaveBatchSize: 1,
				MaxFailures:      int32(nodeCount-1) / 2,
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
		}).(*linhybster)
	}
	return network{
		peers:    peers,
		failures: make(map[*linhybster]struct{}, nodeCount),
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
	n.peers[id] = NewLinHybster(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*linhybster)
}

func (n *network) crash(id peerpb.PeerID) *linhybster {
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

func (n *network) alive(p *linhybster) bool {
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

func (n *network) count(pred func(*linhybster) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*linhybster) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*linhybster) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *linhybster) bool, quorum bool) bool {
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

// waitAcceptInstance waits until the given instance has Accepted.
// If quorum is true, it will wait until the instance is Accepted on
// a quorum of nodes. If it is true, it will wait until the instance
// is Accepted on all nodes.
//func (n *network) waitAcceptInstance(inst *instance, quorum bool) bool {
//	return n.runNetwork(func(p *sdbft) bool {
//		return p.hasAccepted(inst.is.Index)
//	}, quorum)
//}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *linhybster) bool {
		return p.hasExecuted(inst.is.Index)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *linhybster) bool {
		return p.hasPrepared(inst.is.Index)
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
	inst2 := n.peers[0].onRequest(cmd2)
	inst3 := n.peers[0].onRequest(cmd3)

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
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v", p.id, p.log[inst.is.Index])
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

}

func TestManyCommandsMinorityFailures(t *testing.T) {
	count := 19

	n := newNetwork(count)
	n.crashN(n.F(), n.peers[0].id)
	insts := make([]*instance, count*5)
	for i := range insts {
		cmd := command.NewTestingCommand("e")
		insts[i] = n.peers[0].onRequest(cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, true /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v", p.id, p.log[inst.is.Index])
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

}
