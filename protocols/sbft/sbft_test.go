package sbft

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/opentracing/opentracing-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// var (
// 	keysMap = map[string]string{
// 		"4_4": `1664510805692862414738136930728043686959747234055128103437742725804031154265
// 18088117076447549708219333306189605067235947887442227966317388407505221886893
// 4100574137192372358145435629667519579466740500094209864766148497464233411104
// 10503686593798773507167096008980939429999229064630635137359668220332040338658
// 4471718005197823106824379157973642158190995313542447651306999259871250749674
// 12391402968493190671781486321081396438177297533213536842809552759637027149947
// 21465564932047194877163455799126267279964800471415027294562872319151778304875
// 7390568668619100275568223230710602313707005312619210529213308815719853243590`,
// 		"5_6": `14378609814600767568095690351766545743050393520793529698585765364444640571040
// 7387510492061912252669179700239135229964609039089217316986512328353818283247
// 7639342228937080007933886705679176620425997844006464642430108924954189162337
// 13452038929114038791950286328828505022440523223836060358090059662770467021039
// 1471738645046886463025324807839049976737116540673547273180633178801809998672
// 8353873815729214610697197063646343305576303400246399256836678429521389970626
// 6374771142053964026368086592242829833435036740375343744126374485095145167583
// 9336673968527171636894123406653827669147359126704056282856979754499167674374
// 15248670009888693599643498355230827984719989051801487940025104481483071309107
// 6326691351112205302496463121651495015332320935161865301816710744569991623167`,
// 		"13_13": `6355893937383476120080232522015780725139934004882492468393840026656421410348
// 8915690552332061665734015322635268758117979077396797973701111697829907896349
// 12908539370426941816533000213632891230843698438308514470657774613667429089634
// 11650879960976779263395620377313334737672272564821063684221012611151189272547
// 170335925379052947594372294131636281240559193920635104510293906673869785586
// 1787634686222425090876727859487802179590167450356467264640792971924903130699
// 1753930592776127070213114944659026631761634762285812642644875772379881279451
// 474195141927841556559156724680404246794384596538951472584967123525276535599
// 2022685300720746418839750239542270977849923934745881240085877633072824605487
// 20456277352091905450193217873980186700502119974074180343766049063077784073787
// 13884665666906373387402920875231549835041466222825853246320715885265073516005
// 6997107106510846216471080134389467153129312179486506292773256390117361268182
// 16650617699065413956892330721693311596723590967728167970254018310919965200769
// 14781496937901126479421539326538281821845019824181961103117044505807000903552
// 4846726738880600721635973147268373493351598962798745655718308399103454794581
// 1407261136414264038930713585502031369511816636639158776592549494703334177529
// 17803534452831361751661339245629224214729546267888754176977905997397634684771`,
// 	}
// )

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

func readKeys(kType string) ([]string, []string) {
	var skeys []string
	var vkeys []string

	keys, ok := keysMap[kType]
	if !ok {
		log.Fatalf("key file not found: type=%s", kType)
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
	l := logger.NewDefaultLogger()
	skeys, vkeys := readKeys("4_4")
	c := &peer.LocalConfig{
		PeerConfig: &peerpb.PeerConfig{
			SecretKeys:       skeys,
			PublicKeys:       vkeys,
			EnclavePath:      "../../enclaves/threshsign2/enclave_threshsign/",
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
		ID:     3,
		Peers:  []peerpb.PeerID{0, 1, 2, 3},
		Logger: l,
	}
	p := NewSBFT(c).(*sbft)

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

func (s *sbft) ReadMessages() []peerpb.Message {
	msgs := s.msgs
	s.ClearMsgs()
	return msgs
}

func (s *sbft) ExecutableCommands() []peer.ExecPacket {
	cmds := s.executedCmds
	s.ClearExecutedCommands()
	return cmds
}

type conn struct {
	from, to peerpb.PeerID
}

type network struct {
	peers       map[peerpb.PeerID]*sbft
	failures    map[*sbft]struct{}
	dropm       map[conn]float64
	interceptor func(peerpb.PeerID, peerpb.Message)
}

func newNetwork(nodeCount int, fastFailCount int32) network {
	// Start a Datadog tracer, optionally providing a set of options,
	// returning an opentracing.Tracer which wraps it.
	t := opentracer.New(tracer.WithServiceName("go-consensus"))
	defer tracer.Stop() // important for data integrity (flushes any leftovers)

	// Use it with the Opentracing API. The (already started) Datadog tracer
	// may be used in parallel with the Opentracing API if desired.
	opentracing.SetGlobalTracer(t)

	peers := make(map[peerpb.PeerID]*sbft, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", nodeCount-int(fastFailCount), nodeCount))

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Node%d ", r))
		logger.EnableDebug()
		peers[r] = NewSBFT(&peer.LocalConfig{
			PeerConfig: &peerpb.PeerConfig{
				MaxFailures:      int32((nodeCount - int(2*fastFailCount)) / 3),
				MaxFastFailures:  fastFailCount,
				SecretKeys:       skeys,
				PublicKeys:       vkeys,
				EnclavePath:      "../../enclaves/threshsign2/enclave_threshsign/",
				EnclaveBatchSize: 1,
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
		}).(*sbft)
	}
	return network{
		peers:    peers,
		failures: make(map[*sbft]struct{}, nodeCount),
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
	n.peers[id] = NewSBFT(&peer.LocalConfig{
		ID:    p.id,
		Peers: p.nodes,
		//Storage:  p.storage,
		RandSeed: int64(id),
	}).(*sbft)
}

func (n *network) crash(id peerpb.PeerID) *sbft {
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

func (n *network) alive(p *sbft) bool {
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

func (n *network) count(pred func(*sbft) bool) int {
	count := 0
	for _, p := range n.peers {
		if pred(p) {
			count++
		}
	}
	return count
}

func (n *network) quorumHas(pred func(*sbft) bool) bool {
	return n.quorum(n.count(pred))
}

func (n *network) allHave(pred func(*sbft) bool) bool {
	return len(n.peers) == n.count(pred)
}

// runNetwork waits until the given goal for an epaxos node has been
// completed. If quorum is true, it will wait until the goal is completed
// on a quorum of nodes. If it is false, it will wait until the goal is
// completed on all nodes.
func (n *network) runNetwork(goal func(p *sbft) bool, quorum bool) bool {
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

// waitAcceptInstance waits until the given instance has Accepted.
// If quorum is true, it will wait until the instance is Accepted on
// a quorum of nodes. If it is true, it will wait until the instance
// is Accepted on all nodes.
//func (n *network) waitAcceptInstance(inst *instance, quorum bool) bool {
//	return n.runNetwork(func(p *sbft) bool {
//		return p.hasAccepted(inst.is.Index)
//	}, quorum)
//}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitExecuteInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *sbft) bool {
		return p.hasExecuted(inst.is.Index)
	}, quorum)
}

// waitPrePrepareInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *network) waitPrepareInstance(inst *instance, quorum bool) bool {
	return n.runNetwork(func(p *sbft) bool {
		return p.hasPrepared(inst.is.Index)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 0)

	cmd := command.NewTestingCommand("a")
	_, ctx := opentracing.StartSpanFromContext(context.Background(), "test-command")
	inst := n.peers[0].onRequest(ctx, cmd)

	if !n.waitExecuteInstance(inst, true /* quorum */) {
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
	inst := n.peers[0].onRequest(context.TODO(), cmd)

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
	inst := n.peers[0].onRequest(context.TODO(), cmd)

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
		insts[i] = n.peers[0].onRequest(context.TODO(), cmd)
	}

	for _, inst := range insts {
		if !n.waitExecuteInstance(inst, false /* quorum */) {
			for _, p := range n.peers {
				t.Logf("Node %d: instance: %+v; cmd=%v", p.id, p.log[inst.is.Index], p.log[inst.is.Index].is.Command)
			}
			t.Fatalf("command execution failed, instance %+v never installed", inst)
		}
	}

}
