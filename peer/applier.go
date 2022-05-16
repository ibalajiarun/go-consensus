package peer

import (
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/command"
	"github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/ibalajiarun/go-consensus/transport"
)

type ExecPacket struct {
	Cmd      commandpb.Command
	IsSpec   bool
	Meta     []byte
	Callback bool
	NoExec   bool
}

type ExecCallback struct {
	Cmd    commandpb.Command
	Result []byte
}

type asyncApplier struct {
	logger  logger.Logger
	l       sync.Locker
	n       Protocol
	t       transport.Transport
	ct      *command.Tracker
	toApply chan asyncApplyEvent
	toSend  chan asyncSendEvent
	kv      map[string][]byte
	sig     chan<- struct{}
}

type asyncApplyEvent struct {
	ents []ExecPacket
	sync chan struct{}
	stop chan struct{}
}

type asyncSendEvent struct {
	msgs []peerpb.Message
	stop chan struct{}
}

func newAsyncApplier() asyncApplier {
	return asyncApplier{
		toApply: make(chan asyncApplyEvent, 512),
		toSend:  make(chan asyncSendEvent, 512),
		kv:      make(map[string][]byte, 1024*1024),
	}
}

func (pl *asyncApplier) Init(
	logger logger.Logger,
	l sync.Locker,
	n Protocol,
	t transport.Transport,
	ct *command.Tracker,
	notifyChan chan<- struct{},
) {
	pl.logger = logger
	pl.l = l
	pl.n = n
	pl.t = t
	pl.ct = ct
	pl.sig = notifyChan
}

func (pl *asyncApplier) RunOnce() {
	rd := pl.n.MakeReady()
	if sent := pl.trySendMessages(rd.Messages); sent {
		pl.n.ClearMsgs()
	}
	pl.n.ClearExecutedCommands()
	pl.l.Unlock()
	pl.maybeApplyAsync(rd.OrderedCommands)
	pl.l.Lock()
}

func (pl *asyncApplier) maybeApplyAsync(ents []ExecPacket) {
	if len(ents) == 0 {
		return
	}
	// Send to async applier.
	pl.toApply <- asyncApplyEvent{ents: ents}
}

func (pl *asyncApplier) Start() {
	go func() {
		for ev := range pl.toApply {
			if ev.stop != nil {
				close(ev.stop)
				return
			}
			if ev.sync != nil {
				close(ev.sync)
			}
			pl.handleExecutedCmds(ev.ents)
		}
	}()
	go func() {
		for ev := range pl.toSend {
			if ev.stop != nil {
				close(ev.stop)
				return
			}
			sendMessages(pl.t, ev.msgs)
		}
	}()
}

func (pl *asyncApplier) handleExecutedCmds(committed []ExecPacket) {
	for _, pkt := range committed {
		var result *commandpb.CommandResult
		if !pkt.NoExec {
			result = pl.executeCommand(&pkt.Cmd)
			result.Meta = pkt.Meta
			pl.logger.Debugf("Executed command %+v", pkt.Cmd.Timestamp)
		} else {
			if pkt.Callback {
				panic("cannot set callback and noexec at same time")
			}
			result = &commandpb.CommandResult{}
			if err := proto.Unmarshal(pkt.Meta, result); err != nil {
				panic("unable to unmarshal")
			}
		}

		if pkt.Callback {
			msgBytes, err := proto.Marshal(result)
			if err != nil {
				panic(fmt.Sprintf("Unable to marshal msg %v: %v", result, err))
			}
			pl.l.Lock()
			pl.n.Callback(ExecCallback{Cmd: pkt.Cmd, Result: msgBytes})
			pl.l.Unlock()
		} else {
			pl.l.Lock()
			pl.ct.Finish(result)
			pl.l.Unlock()
		}
	}
	pl.notify()
}

func (pl *asyncApplier) executeCommand(cmd *commandpb.Command) *commandpb.CommandResult {
	results := make([]commandpb.OperationResult, len(cmd.Ops))
	for i, op := range cmd.Ops {
		kvOp := op.GetKVOp()
		var val []byte
		if !kvOp.Read {
			pl.kv[kvOp.Key.String()] = kvOp.Value
		} else {
			var present bool
			val, present = pl.kv[kvOp.Key.String()]
			if !present {
				pl.logger.Panic(present)
			}
		}
		results[i] = commandpb.OperationResult{
			Type: &commandpb.OperationResult_KVOpResult{
				KVOpResult: &commandpb.KVOpResult{
					Key:          kvOp.Key,
					Value:        val,
					WriteSuccess: true,
				},
			},
		}
	}
	return &commandpb.CommandResult{
		Timestamp: cmd.Timestamp,
		OpResults: results,
		Target:    cmd.Target,
	}
}

func (pl *asyncApplier) Pause() {
	// Flush toApply channel.
	syncC := make(chan struct{})
	pl.toApply <- asyncApplyEvent{sync: syncC}
	<-syncC
}

func (pl *asyncApplier) Stop() {
	stoppedC := make(chan struct{})
	sendStoppedC := make(chan struct{})
	pl.toApply <- asyncApplyEvent{stop: stoppedC}
	pl.toSend <- asyncSendEvent{stop: sendStoppedC}
	<-stoppedC
	<-sendStoppedC
}

func (pl *asyncApplier) notify() {
	select {
	case pl.sig <- struct{}{}:
	default:
		// Already signaled.
	}
}

func (pl *asyncApplier) trySendMessages(msgs []peerpb.Message) bool {
	if len(msgs) == 0 {
		return false
	}
	select {
	case pl.toSend <- asyncSendEvent{msgs: msgs}:
		return true
	default:
		return false
	}
}

func sendMessages(t transport.Transport, msgs []peerpb.Message) {
	promSendMsgSize.Observe(float64(len(msgs)))
	if len(msgs) > 0 {
		t.Send(msgs)
	}
}
