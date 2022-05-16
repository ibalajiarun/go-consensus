package hybster

import (
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/hybster/hybsterpb"
)

// createAbortState()
func (p *hybster) createAbortState() []pb.Support {
	prs := make([]pb.Support, 0, len(p.omsgs))
	sort.Slice(p.omsgs, func(i, j int) bool {
		return p.omsgs[i].pr.Order < p.omsgs[j].pr.Order
	})
	for _, opr := range p.omsgs {
		prs = append(prs, pb.Support{
			RawMsg: opr.content,
			Cert:   opr.cert,
			From:   opr.from,
		})
	}
	// TODO: add last return if necessary
	return prs
}

// isCorrectAbortState pg. 31
func (p *hybster) isCorrectAbortState(history []pb.Support, fromView pb.View) ([]prPkg, bool) {
	// TODO: add lastCtr if necessary
	prs := make([]prPkg, len(history))
	for i, h := range history {
		prm := pb.HybsterMessage{}
		if err := proto.Unmarshal(h.RawMsg, &prm); err != nil {
			panic(err)
		}
		pr := prm.GetNormal()
		ctr := uint64(uint64(pr.View<<48) | uint64(pr.Order))
		if ok := p.certifier.VerifyIndependentCounterCertificate(h.RawMsg, h.Cert, ctr, 0); !ok {
			p.logger.Panicf("Independent counter verification failed")
			return nil, false
		}
		if pr.View != fromView {
			p.logger.Panicf("view does not match. fromView:%v != pr.View:%v", fromView, pr.View)
			return nil, false
		}
		prs[i] = prPkg{pr, h.RawMsg, h.Cert, h.From}
	}
	return prs, true
}

// installOrderingState pg.31
func (p *hybster) installOrderingState(prps []prPkg) {
	// TODO: do isCorrectOrderingState pg. 31
	// TODO: is it okay to overwrite omsgs? I think no
	p.omsgs = make([]prPkg, 0, len(prps))
	for _, prp := range prps {
		pr := prp.pr
		p.logger.Debugf("installOrderingState for %v", pr)
		inst, ok := p.log[pr.Order]
		if !ok {
			inst = &instance{
				p:           p,
				slot:        pr.Order,
				view:        pr.View,
				command:     pr.Command,
				commandHash: pr.CommandHash,
				quorum:      newQuorum(p),
			}
			p.log[pr.Order] = inst
		} else {
			inst.reset()
			inst.view = pr.View
		}
		// inst.prepared = true
		// inst.quorum.log(prp.from, prp.pr)
		p.omsgs = append(p.omsgs, prp)
	}
}

// abortStateReceived pg.38
func (p *hybster) abortStateReceived(view pb.View, prs []prPkg) {
	p.logger.Debugf("%v == %v", p.stableView, view)
	if p.stableView == view {
		p.installAbortState(prs)
	}
}

// installAbortState pg.31
func (p *hybster) installAbortState(prs []prPkg) {
	// TODO: verify that: isCorrectAbortState is called before all calls to this method
	// TODO: check if I need to update the view for some calls
	p.installOrderingState(prs)
}

func (p *hybster) leaveViewFor(nextView pb.View) {
	if nextView <= p.curView {
		p.logger.Errorf("Ignoring leaving a stale view. Next: %v, Cur: %v", nextView, p.curView)
		return
	}

	prs := p.createAbortState()

	// leaveViewFor
	p.curView = nextView

	// create View Change message
	vc := &pb.ViewChangeMessage{
		FromView: p.stableView,
		ToView:   nextView,
		Order:    p.slot,
		History:  prs,
	}

	p.broadcastVC(vc)
}

// isRelevantViewChange pg.38
func (p *hybster) isRelevantViewChange(m *pb.ViewChangeMessage, from peerpb.PeerID) bool {
	if from == p.id {
		p.logger.Debugf("failed isRelevantViewChange by self %d: %v", from, m)
		return false
	}

	if m.ToView < p.curView || !(p.curView != p.stableView && m.ToView == p.curView) {
		p.logger.Infof("Likely stale VC for lower view: Stable: %d, Current: %d, Given %d", p.stableView, p.curView, m.ToView)
		return false
	}

	return true
}

// isCorrectViewChange pg.37/38
func (p *hybster) isCorrectViewChange(m *pb.ViewChangeMessage, content []byte, cert []byte) ([]prPkg, bool) {
	if m.ToView <= m.FromView {
		p.logger.Errorf("Discarding invalid view transisiton: From: %v, To: %v")
		return nil, false
	}

	// begin isCorrectAbortStateForVC pg. 31
	fromCtr := uint64(uint64(m.FromView<<48) | uint64(m.Order))
	toCtr := uint64(m.ToView << 48)
	if !p.certifier.VerifyContinuingCounterCertificate(content, cert, fromCtr, toCtr, 0) {
		p.logger.Panicf("Continuing counter veficiation failed for %v", m)
		return nil, false
	}

	prs, ok := p.isCorrectAbortState(m.History, m.FromView)
	// end isCorrectAbortStateForVC

	return prs, ok
}

// isCorrectPrepareSet pg. 31
func (p *hybster) isCorrectPrepareSet(history []pb.Support, view pb.View) ([]prPkg, bool) {
	prs := make([]prPkg, len(history))
	for i, h := range history {
		prm := pb.HybsterMessage{}
		if err := proto.Unmarshal(h.RawMsg, &prm); err != nil {
			panic(err)
		}
		pr := prm.GetNormal()
		if pr.View != view {
			p.logger.Debugf("isCorrectViewTransition failed due to pr.view != view, ie. %v != %v, for pr %v", pr.View, view, pr)
			return nil, false
		}
		ctr := uint64(uint64(pr.View<<48) | uint64(pr.Order))
		if ok := p.certifier.VerifyIndependentCounterCertificate(h.RawMsg, h.Cert, ctr, 0); !ok {
			panic("verification failed")
		}
		prs[i] = prPkg{pr, h.RawMsg, h.Cert, h.From}
	}
	return prs, true
}

// isCorrectViewTransition pg. 32
func (p *hybster) isCorrectViewTransition(history []pb.Support, view pb.View, abrtPrPkgs []prPkg) ([]prPkg, bool) {
	prpkgs, ok := p.isCorrectPrepareSet(history, view)
	if !ok {
		return nil, false
	}

	p.logger.Debugf("isCorrectViewTransition %v == %v", len(prpkgs), len(abrtPrPkgs))

	// if len(prpkgs) != len(abrtPrPkgs) {
	// 	return nil, false
	// }
	// TODO: i dont think search is really good here. may be merge abrtPrPkgs?
	for _, prp := range prpkgs {
		i := sort.Search(len(abrtPrPkgs), func(i int) bool {
			// if prp.pr.Order == abrtPrPkgs[i].pr.Order {
			// 	return abrtPrPkgs[i].pr.View >= prp.pr.View
			// }
			return abrtPrPkgs[i].pr.Order >= prp.pr.Order
		})
		if i == len(abrtPrPkgs) {
			return nil, false
		}
	}

	return prpkgs, true
}

// isCorrectViewChangeCertificate pg.38
func (p *hybster) isCorrectViewChangeCertificate(vcPkgs []vcPkg, from, to pb.View) ([]vcPkg, []prPkg, bool) {
	if len(vcPkgs) < p.f+1 {
		return nil, nil, false
	}

	vcpsQ := make([]vcPkg, 0, 10)
	prpsQ := make([]prPkg, 0, 10)
	for _, vcp := range vcPkgs {
		if vcp.vc.FromView > from {
			continue
		}
		if vcp.vc.ToView != to {
			continue
		}
		prpkgs, ok := p.isCorrectViewChange(vcp.vc, vcp.content, vcp.cert)
		if !ok {
			continue
		}
		vcpsQ = append(vcpsQ, vcp)
		toAdd := make([]prPkg, 0, 10)
		p.logger.Debugf("isCorrectViewChange %v", vcp.vc)
		for _, prp := range prpkgs {
			p.logger.Debugf("prp pr %v;", prp.pr)
			i := sort.Search(len(prpsQ), func(i int) bool {
				return prp.pr.Order < prpsQ[i].pr.Order ||
					prp.pr.View < prpsQ[i].pr.View
			})
			if i == len(prpsQ) {
				toAdd = append(toAdd, prp)
			}
		}
		prpsQ = append(prpsQ, toAdd...)
		sort.Slice(prpsQ, func(i, j int) bool {
			return prpsQ[i].pr.Order < prpsQ[j].pr.Order ||
				prpsQ[i].pr.View < prpsQ[j].pr.View
		})
	}
	if len(vcpsQ) > p.f {
		return vcpsQ, prpsQ, true
	}
	return nil, nil, false
}

// isCorrectNewViewAck pg.41
// TODO: check if fromView passed is correct
func (p *hybster) isCorrectNewViewAck(nvap nvaPkg, fromView pb.View) ([]prPkg, bool) {
	if !p.certifier.VerifyContinuingCounterCertificate(nvap.content, nvap.cert, 0, 0, 2) {
		p.logger.Errorf("Continuing counter veficiation failed for %v", nvap.nva)
		return nil, false
	}
	prs, ok := p.isCorrectAbortState(nvap.nva.History, fromView)
	if !ok {
		p.logger.Errorf("isCorrectAbortState failed for %v", nvap.nva)
		return nil, false
	}
	return prs, true
}

// isCorrectNewViewCertificate pg. 39
func (p *hybster) isCorrectNewViewCertificate(vcPkgs []vcPkg, nvAcks []nvaPkg, from, to pb.View) ([]vcPkg, []prPkg, bool) {
	vcpsQ, prpsQ, ok := p.isCorrectViewChangeCertificate(vcPkgs, from, to)
	if !ok {
		return nil, nil, false
	}

	p.logger.Debugf("prpsQ from isCorrectViewChangeCertificate: %v", len(prpsQ))

	// TODO: should i check if len(nvAcks) == 1 since I am implementing discarding?
	for _, nvap := range nvAcks {
		prs, ok := p.isCorrectNewViewAck(nvap, from)
		if !ok {
			return nil, nil, false
		}
		toAdd := make([]prPkg, 0, len(prs))
		for _, prp := range prs {
			i := sort.Search(len(prpsQ), func(i int) bool {
				return prp.pr.Order < prpsQ[i].pr.Order ||
					prp.pr.View < prpsQ[i].pr.View
			})
			if i == len(prpsQ) {
				toAdd = append(toAdd, prp)
			}
		}
		prpsQ = append(prpsQ, toAdd...)
		sort.Slice(prpsQ, func(i, j int) bool {
			return prpsQ[i].pr.Order < prpsQ[j].pr.Order ||
				prpsQ[i].pr.View < prpsQ[j].pr.View
		})
	}

	p.logger.Debugf("prpsQ from nvack: %v", len(prpsQ))

	vcCount := 0
	for _, vc := range vcPkgs {
		if vc.vc.FromView == from && vc.vc.ToView == to {
			vcCount++
		}
	}
	for _, nvap := range nvAcks {
		// TODO: remove this check. add discardNVA() method
		if from <= nvap.nva.View && nvap.nva.View <= to {
			vcCount++
		}
	}

	return vcpsQ, prpsQ, vcCount > p.f
}

func (p *hybster) onViewChange(m *pb.ViewChangeMessage, content, cert []byte, from peerpb.PeerID) {
	if !p.isRelevantViewChange(m, from) {
		p.logger.Infof("isRelevantViewChange failed for %v", m)
		return
	}

	prs, ok := p.isCorrectViewChange(m, content, cert)
	if !ok {
		p.logger.Panicf("isCorrectViewChange failed for %v", m)
		return
	}

	p.installViewChange(vcPkg{m, content, cert, from}, prs)

	// Task: support if enough nodes want a view change
	if p.curView == p.stableView && m.ToView > p.curView {
		count := 0
		for _, vcp := range p.vmsgs {
			if vcp.vc.ToView == m.ToView {
				count++
			}
		}
		if count > len(p.nodes)/2 {
			p.leaveViewFor(m.ToView)
		}
	}

	p.trySendNewView(m.FromView, m.ToView)
}

// createViewTransition pg. 32/33
func (p *hybster) createViewTransition(toView pb.View, prpkgs []prPkg) ([]prPkg, []pb.Support) {
	sort.Slice(prpkgs, func(i, j int) bool {
		return prpkgs[i].pr.Order < prpkgs[j].pr.Order
	})

	prsBytes := make([]pb.Support, 0, len(prpkgs))
	outprs := make([]prPkg, 0, len(prpkgs))
	for _, opr := range prpkgs {
		pr := opr.pr
		pr.View = toView

		idx := sort.Search(len(outprs), func(i int) bool {
			return outprs[i].pr.Order >= pr.Order
		})
		if idx < len(outprs) && outprs[idx].pr.Order == pr.Order {
			continue
		}

		p.logger.Debugf("createViewTransition %v", pr)

		mBytes, cert := p.marshalAndSignNormal(pr)
		outprs = append(outprs, prPkg{pr, mBytes, cert, p.id})
		prsBytes = append(prsBytes, pb.Support{RawMsg: mBytes, Cert: cert, From: p.id})
	}

	return outprs, prsBytes
}

// task newLeaderViewReady pg. 39
func (p *hybster) trySendNewView(fromView, toView pb.View) {
	// TODO: validate and check for NewViewMessage in the quorum
	if p.leaderOfView(toView) == p.id && p.curView <= toView {
		// TODO: ^^^ && p.stableView != p.curView ???

		// isCorrectNewViewCertificate
		vcpsQ, prpsQ, ok := p.isCorrectNewViewCertificate(p.vmsgs, p.nvamsgs, fromView, toView)
		if !ok {
			return
		}

		// begin establishView pg. 40
		if toView > p.curView {
			// TODO: forward counter
			p.curView = toView
		}

		if toView <= p.stableView {
			p.logger.Panicf("New view is less than stable view? staleView: %d, NewView %d", p.stableView, toView)
			return
		}

		// allAbortStates
		// prpkgs := p.allAbortStates(p.vmsgs, p.nvamsgs)

		// createViewTransition
		prps, prsBytes := p.createViewTransition(toView, prpsQ)

		vcMsgsQ := make([]pb.Support, len(vcpsQ))
		for i, vcp := range vcpsQ {
			vcMsgsQ[i] = pb.Support{RawMsg: vcp.content, Cert: vcp.cert, From: vcp.from}
		}

		nv := pb.NewViewMessage{
			FromView: fromView,
			ToView:   toView,
			History:  prsBytes,
			Proof:    vcMsgsQ,
		}

		p.broadcastNV(nv)

		p.enterView(toView, prps)

		for _, prp := range prps {
			pr := prp.pr
			inst, ok := p.log[pr.Order]
			if !ok {
				p.logger.Panicf("inst not found %v", pr.Order)
			}
			inst.prepared = true
			inst.quorum.log(prp.from, prp.pr)
		}
		// end establishView
	}
}

// latestKnownView pg. 40
func (p *hybster) latestKnownView() pb.View {
	max := pb.View(0)
	for _, nv := range p.nvmsgs {
		if max < nv.ToView {
			max = nv.ToView
		}
	}
	return max
}

// enterView pg.32,41
func (p *hybster) enterView(view pb.View, prps []prPkg) {
	if view <= p.stableView {
		p.logger.Panicf("New view is less than stable view? staleView: %d, NewView %d", p.stableView, view)
	}

	if p.curView < view {
		p.logger.Panicf("How come I did not sign up for view change? curView: %d, NewView %d", p.curView, view)
	}

	p.logger.Infof("Entering View %v", view)
	p.stableView = view

	p.installOrderingState(prps)
}

// newFollowerViewReady pg.40
func (p *hybster) onNewViewReady(m *pb.NewViewMessage) {
	p.logger.Debugf("trySendNewViewAck %v %v %v", m.ToView, p.latestKnownView(), p.stableView)
	if m.ToView == p.latestKnownView() && m.ToView > p.stableView {
		belated := m.ToView < p.curView

		if m.ToView > p.curView {
			// TODO: abortView
			p.curView = m.ToView
		}

		prs, ok := p.isCorrectPrepareSet(m.History, m.ToView)
		if !ok {
			p.logger.Panicf("isCorrectPrepareSet %v", m)
		}

		p.enterView(m.ToView, prs)

		if belated {
			// TODO: i am changing p.curView to m.ToView. Verify
			nva := &pb.NewViewAckMessage{
				View:    m.ToView,
				History: m.History,
			}

			p.broadcastNVA(nva)
		}
	}
}

// isRelevantNewView pg.40
func (p *hybster) isRelevantNewView(m *pb.NewViewMessage) bool {
	return m.ToView > p.latestKnownView()
}

// isCorrectNewView pg.40
func (p *hybster) isCorrectNewView(nvp nvPkg, from peerpb.PeerID) ([]vcPkg, []prPkg, bool) {
	m := nvp.nv
	if p.leaderOfView(m.ToView) != from {
		p.logger.Panicf("Received NewView message from non-leader %v", m)
		return nil, nil, false
	}

	// hasValidCertificate
	counter := uint64(m.ToView << 48)
	if !p.certifier.VerifyIndependentCounterCertificate(nvp.content, nvp.cert, counter, 1) {
		panic("Invalid certificate")
	}

	vcPkgs := make([]vcPkg, len(m.Proof))
	for i, p := range m.Proof {
		vcm := &pb.HybsterMessage{}
		if err := proto.Unmarshal(p.RawMsg, vcm); err != nil {
			panic(err)
		}
		vc := vcm.GetViewChange()
		vcPkgs[i] = vcPkg{vc, p.RawMsg, p.Cert, p.From}
	}

	_, abortPrs, ok := p.isCorrectNewViewCertificate(vcPkgs, p.nvamsgs, m.FromView, m.ToView)
	if !ok {
		p.logger.Errorf("invalid isCorrectNewViewCertificate")
		return nil, nil, false
	}

	// TODO: call allAbortStates() and send that instead of m.History
	// abortPrs := p.allAbortStates(vcPkgs, p.nvamsgs)
	prPkgs, ok := p.isCorrectViewTransition(m.History, m.ToView, abortPrs)
	if !ok {
		p.logger.Errorf("invalid isCorrectViewTransition")
		return nil, nil, false
	}

	return vcPkgs, prPkgs, true
}

// installViewChange pg. 38
func (p *hybster) installViewChange(vcp vcPkg, prs []prPkg) {
	// storeViewChange
	// TODO: think to properly store viewchanges
	p.vmsgs = append(p.vmsgs, vcp)

	p.abortStateReceived(vcp.vc.ToView, prs)
}

// receive NewView pg.40
func (p *hybster) onNewView(m *pb.NewViewMessage, content, cert []byte, from peerpb.PeerID) {
	p.logger.Debugf("onNewView from %d: %v", from, m)

	// isRelevantNewView
	if !p.isRelevantNewView(m) {
		p.logger.Panicf("Invalid NewView %m", m)
		return
	}

	// isCorrectNewView
	vcps, prs, ok := p.isCorrectNewView(nvPkg{m, content, cert}, from)
	if !ok {
		p.logger.Panicf("isCorrectNewView failed for %v", m)
		return
	}

	// TODO: check if this is correct
	if p.curView == p.stableView && p.curView == m.ToView {
		p.logger.Debugf("I am already in that view. Ignoring new view message %v", m)
		return
	}

	// begin installNewView pg. 40

	// storeNewView
	p.nvmsgs = append(p.nvmsgs, *m)

	// task newFollowerViewReady pg.40
	p.onNewViewReady(m)

	p.logger.Debugf("len %v", len(vcps))
	for _, vcp := range vcps {
		if !p.isRelevantViewChange(vcp.vc, vcp.from) {
			p.logger.Debugf("onNewView's isRelevantViewChange failed for %v", vcp.vc)
			continue
		}
		p.logger.Debugf("calling installViewChange %v", prs)
		p.installViewChange(vcp, prs)
	}

	// end installNewView pg.40

	for _, prp := range p.omsgs {
		p.onPrepare(prp.pr, from)
	}
}

func (p *hybster) isRelevantNewViewAck(m *pb.NewViewAckMessage) bool {
	if m.View < p.stableView {
		return false
	}

	for _, nvap := range p.nvamsgs {
		if m.View <= nvap.nva.View {
			p.logger.Debugf("Higher NVA already present %v", m)
			return false
		}
	}

	return true
}

// onNewViewAck pg.41
func (p *hybster) onNewViewAck(m *pb.NewViewAckMessage, content, cert []byte, from peerpb.PeerID) {
	// isRelevantNewViewAck
	if !p.isRelevantNewViewAck(m) {
		p.logger.Debugf("isRelevantNewViewAck failed for %m")
		return
	}

	nvap := nvaPkg{m, content, cert, from}

	// isCorrectNewViewAck
	_, ok := p.isCorrectNewViewAck(nvap, m.View)
	if !ok {
		p.logger.Debugf("isCorrectNewViewAck failed for %v", m)
		return
	}

	// begin installNewViewAck pg.41
	p.nvamsgs = append(p.nvamsgs, nvap)

	prs, ok := p.isCorrectAbortState(m.History, m.View)
	if !ok {
		p.logger.Debugf("isCorrectAbortState failed for %v", m)
		return
	}
	p.abortStateReceived(m.View, prs)
	// end installNewViewAck pg.41

	p.trySendNewView(m.View, p.curView)
}
