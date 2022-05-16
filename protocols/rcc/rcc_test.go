package rcc

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/pkg/testing/network"
)

type rccNetwork struct {
	network.BaseNetwork
	rccPeers map[peerpb.PeerID]*RCC
}

func newNetwork(nodeCount int, failures int, algorithm peerpb.Algorithm) rccNetwork {
	peers := make(map[peerpb.PeerID]peer.Protocol, nodeCount)
	rccPeers := make(map[peerpb.PeerID]*RCC, nodeCount)
	peersSlice := make([]peerpb.PeerID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		peersSlice[i] = peerpb.PeerID(i)
	}
	skeys, vkeys := readKeys(nodeCount)

	peerConfig := &peerpb.PeerConfig{
		SecretKeys:  skeys,
		PublicKeys:  vkeys,
		EnclavePath: "/home/balaji/workplace/mygo/go-consensus/enclaves/threshsign/enclave_threshsign/",
		MaxFailures: int32(failures),
		Workers: map[string]uint32{
			"mac_sign":   1,
			"tss_agg":    1,
			"tss_verify": 1,
		},
		WorkersQueueSizes: map[string]uint32{
			"mac_sign":   300,
			"tss_agg":    32,
			"tss_verify": 32,
		},
		RccAlgorithm: algorithm,
	}

	for _, r := range peersSlice {
		logger := logger.NewDefaultLoggerWithPrefix(fmt.Sprintf("Peer%d ", r))
		logger.EnableDebug()
		p := NewRCC(&peer.LocalConfig{
			PeerConfig: peerConfig,
			ID:         r,
			Peers:      peersSlice,
			RandSeed:   int64(r),
			Logger:     logger,
		})
		peers[r], rccPeers[r] = p, p
	}
	return rccNetwork{
		BaseNetwork: network.NewNetwork(peers, failures, 10000),
		rccPeers:    rccPeers,
	}
}

// waitExecuteInstance waits until the given instance has executed.
// If quorum is true, it will wait until the instance is executed on
// a quorum of nodes. If it is true, it will wait until the instance
// is executed on all nodes.
func (n *rccNetwork) waitExecuteInstance(cmd *commandpb.Command, quorum bool) bool {
	return n.RunNetwork(func(p peer.Protocol) bool {
		return p.(*RCC).hasExecuted(cmd)
	}, quorum)
}

// TestExecuteCommandsNoFailures verifies that the primary replica can propose a
// command and that the command will be executed, in the case where there
// are no failures.
func TestExecuteCommandsNoFailures(t *testing.T) {
	n := newNetwork(4, 1, peerpb.Algorithm_PBFT)

	cmd := command.NewTestingCommand("a")
	n.rccPeers[0].Request(cmd)

	if !n.waitExecuteInstance(cmd, false /* quorum */) {
		t.Fatalf("command execution failed, cmd %+v never installed", cmd)
	}
}

func TestManyCommandsNoFailures(t *testing.T) {
	count := 13
	failures := 4

	n := newNetwork(count, failures, peerpb.Algorithm_PBFT)
	cmds := make([]*commandpb.Command, count*100)
	for i := range cmds {
		cmd := command.NewTestingCommand("e")
		target := peerpb.PeerID(i % count)
		cmd.Target = target
		cmds[i] = cmd
		n.rccPeers[target].Request(cmd)
	}

	for _, cmd := range cmds {
		if !n.waitExecuteInstance(cmd, false /* quorum */) {
			t.Fatalf("command execution failed, instance %+v never installed", cmd)
		}
	}
}

var (
	keysMap = map[int]string{
		4: `3872398145671381750051310361161029866141599796961754550720954525373154029641
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
