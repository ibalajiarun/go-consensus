package dispel

import "github.com/ibalajiarun/go-consensus/protocols/dispel/dispelpb"

func (d *Dispel) exec() {
	for {
		ep, ok := d.epochs[d.nextDeliverEpoch]
		if !ok || ep.rbDoneCount < d.rbThreshold || ep.consDoneCount != len(d.peers) {
			if ok {
				d.logger.Debugf("not committed %v: %v < %v; %v != %v", d.nextDeliverEpoch, ep.rbDoneCount, d.rbThreshold, ep.consDoneCount, len(d.peers))
				for id, cons := range ep.cons {
					d.logger.Debugf("not committed status: cons %d, decided %v.", id, cons.decided)
				}
			}
			break
		}

		for id, cons := range ep.cons {
			if cons.decided && cons.decidedValue == 1 &&
				ep.rbs[id].rbs.Status != dispelpb.RBState_Readied {
				return
			}
		}

		d.logger.Debugf("Replica %d execute epoch %v\n", d.id, ep.epochNum)

		for id, cons := range ep.cons {
			d.logger.Debugf("execute: cons %d, decided %v, command status: %v", id, cons.decidedValue, ep.rbs[id])
			if cons.decided && cons.decidedValue == 1 {
				d.enqueueForExecution(ep.rbs[id].rbs.Command)
				ep.rbs[id].rbs.Command = nil
			} else if rb, ok := ep.rbs[id]; id == d.id && cons.decided && ok {
				d.logger.Debugf("reproposing %v\n", rb.rbs.Command.Timestamp)
				rb.reproposed = true
				d.onRequest(rb.rbs.Command)
				promReproposeCount.Inc()
			}
		}
		d.nextDeliverEpoch++
	}
}
