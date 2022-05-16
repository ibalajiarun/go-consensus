package dispel

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/ibalajiarun/go-consensus/peer"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"
)

type dbftRound struct {
	roundNum pb.Round
	est      int32

	binValues mapset.Set

	bvbSent      map[int32]bool
	bvbEstimates map[int32]int

	coordStepDone     bool
	coordTimerExpired bool
	coordMessage      *pb.ConsensusMessage

	auxEstimates []mapset.Set
	auxSent      bool

	cons *dbft
}

type dbft struct {
	peerID peerpb.PeerID

	rs map[pb.Round]*dbftRound

	decided            bool
	decidedValue       int32
	decidedRound       pb.Round
	halt               bool
	restartOnBinValues bool

	timeout int

	epoch *epoch
}

func makeCons(ep *epoch, pID peerpb.PeerID) *dbft {
	return &dbft{
		peerID: pID,
		rs:     make(map[pb.Round]*dbftRound),
		epoch:  ep,
	}
}

func makeRound(cons *dbft, rNum pb.Round) *dbftRound {
	return &dbftRound{
		roundNum:     rNum,
		binValues:    mapset.NewThreadUnsafeSet(),
		bvbEstimates: make(map[int32]int),
		bvbSent:      make(map[int32]bool),
		cons:         cons,
	}
}

func (d *Dispel) tryStartConsensus(ep *epoch) {
	if ep.rbDoneCount != d.rbThreshold {
		return
	}

	for _, id := range d.peers {
		cons, ok := ep.cons[id]
		if !ok {
			cons = makeCons(ep, id)
			ep.cons[id] = cons
		}

		rb, ok := ep.rbs[id]
		if ok && rb.rbs.Status == pb.RBState_Readied {
			d.binaryPropose(cons, 1, -1)
		} else {
			d.binaryPropose(cons, 1, 0)
		}
	}
}

func (d *Dispel) onRBDone(ep *epoch, id peerpb.PeerID) {
	if ep.rbDoneCount <= d.rbThreshold {
		return
	}

	cons, ok := ep.cons[id]
	if !ok {
		d.logger.Panicf("RB not found: (%v, %v)", ep.epochNum, id)
	}

	d.binaryPropose(cons, 1, -1)
}

func (d *Dispel) binaryPropose(cons *dbft, rndNum pb.Round, est int32) {
	if cons.halt {
		d.logger.Debugf("halted... peer_id: %v, decided_round: %v", cons.peerID, cons.decidedRound)
		return
	}

	d.logger.Debugf("binaryPropose cons: %v, rnd %v, est %v", cons.peerID, rndNum, est)

	rs, ok := cons.rs[rndNum]
	if !ok {
		rs = makeRound(cons, rndNum)
		cons.rs[rndNum] = rs
	}

	if est == -1 {
		rs.est = 1
		rs.binValues.Add(int32(1))
		rs.coordStepDone = true

		d.broadcastAux(rs, rs.binValues)
	} else {
		rs.est = est
		if !rs.bvbSent[est] {
			rs.bvbSent[est] = true
			rs.bvbEstimates[est]++

			bvb := &pb.ConsensusMessage{
				EpochNum: cons.epoch.epochNum,
				PeerID:   cons.peerID,
				RoundNum: rs.roundNum,
				Type:     pb.ConsensusMessage_Estimate,
				Estimate: rs.est,
			}

			d.broadcast(bvb, false)
		}
	}
}

func (d *Dispel) onBVBroadcast(m *pb.ConsensusMessage, from peerpb.PeerID) {
	ep, ok := d.epochs[m.EpochNum]
	if !ok {
		d.logger.Panicf("unknown epoch %v", m)
	}

	cons, ok := ep.cons[m.PeerID]
	if !ok {
		cons = makeCons(ep, m.PeerID)
		ep.cons[m.PeerID] = cons
	}

	rs, ok := cons.rs[m.RoundNum]
	if !ok {
		rs = makeRound(cons, m.RoundNum)
		cons.rs[m.RoundNum] = rs
	}

	rs.bvbEstimates[m.Estimate]++

	count := rs.bvbEstimates[m.Estimate]
	if count == d.maxFailures+1 {
		if !rs.bvbSent[m.Estimate] {
			rs.bvbSent[m.Estimate] = true
			d.broadcast(m, false)
		}
	}

	if count == 2*d.maxFailures+1 {
		rs.binValues.Add(m.Estimate)
		d.afterBVBroadcast(rs)
	}
}

func (d *Dispel) afterBVBroadcast(rs *dbftRound) {
	rs.cons.timeout = rs.cons.timeout + 1

	if !rs.coordStepDone {
		rs.coordStepDone = true
		coord := (int(rs.roundNum) - 1) % len(d.peers)
		if int(d.id) == coord {
			if rs.binValues.Cardinality() != 1 {
				d.logger.Panicf("length is more than one")
			}
			w := rs.binValues.Pop().(int32)
			rs.binValues.Add(w)

			cv := &pb.ConsensusMessage{
				EpochNum: rs.cons.epoch.epochNum,
				PeerID:   rs.cons.peerID,
				RoundNum: rs.roundNum,
				Type:     pb.ConsensusMessage_CoordValue,
				Estimate: w,
			}
			d.broadcast(cv, true)
		}

		const coordTimeout = 5
		coordTimer := peer.MakeTickingTimer(coordTimeout*rs.cons.timeout, func() {
			d.onCoordTimerExpired(rs)
		})
		d.registerOneTimeTimer(coordTimer)

		d.onAfterCoordSentAndBinValuesNotNull(rs)
	}

	if rs.cons.restartOnBinValues &&
		rs.binValues.Contains(int32(0)) &&
		rs.binValues.Contains(int32(1)) {

		rs.cons.restartOnBinValues = false
		d.binaryPropose(rs.cons, rs.roundNum+1, rs.est)
	}
}

func (d *Dispel) onCoordValue(m *pb.ConsensusMessage, from peerpb.PeerID) {
	ep, ok := d.epochs[m.EpochNum]
	if !ok {
		d.logger.Panicf("unknown epoch %v", m)
	}

	cons, ok := ep.cons[m.PeerID]
	if !ok {
		cons = makeCons(ep, m.PeerID)
		ep.cons[m.PeerID] = cons
	}

	rs, ok := cons.rs[m.RoundNum]
	if !ok {
		rs = makeRound(cons, m.RoundNum)
		cons.rs[m.RoundNum] = rs
	}

	rs.coordMessage = m
}

func (d *Dispel) onAfterCoordSentAndBinValuesNotNull(rs *dbftRound) {
	if !rs.coordTimerExpired {
		return
	}

	d.onAfterBinValuesNotNullAndCoordTimerExpires(rs)
}

func (d *Dispel) onCoordTimerExpired(rs *dbftRound) {
	rs.coordTimerExpired = true
	if rs.binValues.Cardinality() < 1 {
		return
	}

	d.onAfterBinValuesNotNullAndCoordTimerExpires(rs)
}

func (d *Dispel) onAfterBinValuesNotNullAndCoordTimerExpires(rs *dbftRound) {
	// if coord with value w is received and w \in bin_values
	// then use w as aux value

	auxVal := rs.binValues
	if rs.coordMessage != nil {
		if rs.binValues.Contains(rs.coordMessage) {
			auxVal = mapset.NewThreadUnsafeSet()
			auxVal.Add(rs.coordMessage.Estimate)
		}
	}

	d.broadcastAux(rs, auxVal)
}

func (d *Dispel) broadcastAux(rs *dbftRound, auxVal mapset.Set) {
	if rs.auxSent {
		return
	}
	am := &pb.ConsensusMessage{
		EpochNum: rs.cons.epoch.epochNum,
		PeerID:   rs.cons.peerID,
		RoundNum: rs.roundNum,
		Type:     pb.ConsensusMessage_Aux,
		Aux:      toInt32Slice(auxVal.ToSlice()),
	}
	rs.auxSent = true
	d.broadcast(am, true)
	d.onAfterSendReceiveAux(rs)
}

func (d *Dispel) onAuxMessage(m *pb.ConsensusMessage, from peerpb.PeerID) {
	ep, ok := d.epochs[m.EpochNum]
	if !ok {
		d.logger.Panic("epoch not found %v", m)
	}

	cons, ok := ep.cons[m.PeerID]
	if !ok {
		cons = makeCons(ep, m.PeerID)
		ep.cons[m.PeerID] = cons
	}

	rs, ok := cons.rs[m.RoundNum]
	if !ok {
		rs = makeRound(cons, m.RoundNum)
		cons.rs[m.RoundNum] = rs
	}

	bvalSet := mapset.NewThreadUnsafeSet()
	for i := range m.Aux {
		bvalSet.Add(m.Aux[i])
	}
	rs.auxEstimates = append(rs.auxEstimates, bvalSet)

	d.onAfterSendReceiveAux(rs)
}

func (d *Dispel) onAfterSendReceiveAux(rs *dbftRound) {
	if !rs.auxSent {
		d.logger.Debugf("Aux not sent yet: epoch: %v, cons: %v, rnd: %v",
			rs.cons.epoch.epochNum, rs.cons.peerID, rs.roundNum)
		return
	}

	if len(rs.auxEstimates) > 2*d.maxFailures {
		d.logger.Debugf("majority aux received: epoch: %v, cons: %v, rndNum: %v, rnd: %v",
			rs.cons.epoch.epochNum, rs.cons.peerID, rs.roundNum, rs)

		valuesSet := mapset.NewThreadUnsafeSet()
		for _, set := range rs.auxEstimates {
			valuesSet = valuesSet.Union(set)
		}

		if !valuesSet.IsSubset(rs.binValues) {
			d.logger.Debugf("not a subset: (%v, %v, %v); valuesSet %v; binValues %v",
				rs.cons.epoch.epochNum, rs.cons.peerID, rs.roundNum, valuesSet, rs.binValues)
			return
		}

		cons := rs.cons
		b := rs.roundNum % 2
		if valuesSet.Cardinality() == 1 {
			rs.est = valuesSet.Pop().(int32)
			rs.binValues.Add(rs.est)
			if rs.est == int32(b) && !cons.decided {
				d.logger.Debugf("Decided: Epoch: %v, ID: %v, Round: %v, Estimate: %v", cons.epoch.epochNum, cons.peerID, rs.roundNum, rs.est)
				cons.decided = true
				cons.decidedValue = rs.est
				cons.epoch.consDoneCount++
				cons.decidedRound = rs.roundNum
			}
		} else {
			rs.est = int32(b)
		}

		if cons.decided && cons.decidedRound == rs.roundNum {
			d.exec()
			if rs.binValues.Contains(int32(0)) && rs.binValues.Contains(int32(1)) {
				d.binaryPropose(cons, rs.roundNum+1, rs.est)
			} else {
				cons.restartOnBinValues = true
			}
		} else if cons.decided && cons.decidedRound == rs.roundNum-2 {
			cons.halt = true
		} else {
			d.binaryPropose(cons, rs.roundNum+1, rs.est)
		}
	}
}

func toInt32Slice(s []interface{}) []int32 {
	out := make([]int32, len(s))
	for i, d := range s {
		out[i] = d.(int32)
	}
	return out
}
