package prime

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/prime/primepb"
)

type oinstance struct {
	s  *prime
	is pb.OInstanceState

	pCert *oquorum
	cCert *oquorum
}

func makeOInstance(m *pb.OrderMessage, p *prime) *oinstance {
	return &oinstance{
		is: pb.OInstanceState{
			View:            m.View,
			Index:           m.Index,
			Status:          pb.OInstanceState_None,
			POSummaryMatrix: m.POSummaryMatrix,
		},
		pCert: newOQuorum(p),
		cCert: newOQuorum(p),
	}
}

func (p *prime) updateOCommand(inst *oinstance, m *pb.OrderMessage) bool {
	if inst.is.POSummaryMatrix != nil {
		for i := range inst.is.POSummaryMatrix.POSummaryMatrix {
			if !inst.is.POSummaryMatrix.POSummaryMatrix[i].Equals(m.POSummaryMatrix.POSummaryMatrix[i]) {
				p.logger.Debugf("Different summaries for same instance %v: Has: %v, Received %v.", inst.is, inst.is.POSummaryMatrix, m.POSummaryMatrix)
				return false
			}
		}
	} else {
		inst.is.POSummaryMatrix = m.POSummaryMatrix
	}
	return true
}

func (p *prime) updateOCommandHash(inst *oinstance, m *pb.OrderMessage) bool {
	if inst.is.POSummaryMatrixHash != nil {
		if !bytes.Equal(inst.is.POSummaryMatrixHash, m.POSummaryMatrixHash) {
			p.logger.Debugf("Different summary hashes for same instance %v: Has: %v, Received %v.", inst.is, inst.is.POSummaryMatrixHash, m.POSummaryMatrixHash)
			return false
		}
	} else {
		inst.is.POSummaryMatrixHash = m.POSummaryMatrixHash
	}
	return true
}

func (p *prime) sendPrePrepare() {
	if !p.isPrimaryAtView(p.id, p.oview) {
		p.logger.Errorf("Not the leader of view %d: %d", p.oview, p.id)
		return
	}

	if p.lastPrepareHash != nil {
		if bytes.Equal(p.lastPreorderSummariesHash, p.lastPrepareHash) {
			return
		}
	}

	p.prePrepareCounter++

	mat := make([]pb.POSummary, len(p.lastPreorderSummaries.POSummaryMatrix))
	copy(mat, p.lastPreorderSummaries.POSummaryMatrix)
	for i := range mat {
		copy(mat[i].POSummary, p.lastPreorderSummaries.POSummaryMatrix[i].POSummary)
	}

	poSumMat := pb.POSummaryMatrix{POSummaryMatrix: mat}

	summaryBytes, err := proto.Marshal(&poSumMat)
	if err != nil {
		panic(err)
	}
	summaryHash := sha256.Sum256(summaryBytes)

	p.lastPrepareHash = p.lastPreorderSummariesHash

	index := p.oindex
	p.oindex++

	inst := &oinstance{
		s: p,
		is: pb.OInstanceState{
			View:                p.oview,
			Index:               index,
			Status:              pb.OInstanceState_PrePrepared,
			POSummaryMatrix:     &poSumMat,
			POSummaryMatrixHash: summaryHash[:],
		},
		pCert: newOQuorum(p),
		cCert: newOQuorum(p),
	}
	p.olog[index] = inst

	p.logger.Debugf("Sending PrePrepare for %v", inst.is.Index)
	pm := &pb.OrderMessage{
		View:                inst.is.View,
		Index:               inst.is.Index,
		Type:                pb.OrderMessage_PrePrepare,
		POSummaryMatrix:     inst.is.POSummaryMatrix,
		POSummaryMatrixHash: inst.is.POSummaryMatrixHash,
	}
	inst.pCert.log(p.id, pm)
	p.broadcast(pm, false)
}

func (p *prime) checkOCommandHash(cmd *pb.POSummaryMatrix, hash []byte) bool {
	cmdBytes, err := proto.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	compHash := sha256.Sum256(cmdBytes)
	return bytes.Equal(hash, compHash[:])
}

func (p *prime) onPrePrepare(m *pb.OrderMessage, from peerpb.PeerID) {
	// Check if the message sender is the current primary
	if !p.isCurrentPrimary(from) {
		p.logger.Errorf("PrePrepare vcSender Non-Primary %v: %v", from, m)
		return
	}

	if !p.checkOCommandHash(m.POSummaryMatrix, m.POSummaryMatrixHash) {
		p.logger.Panicf("The hash does not correspond to the command: %v", m)
	}

	inst, exists := p.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, p)
		p.olog[m.Index] = inst
	} else if !p.updateOCommand(inst, m) || !p.updateOCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	inst.pCert.log(from, m)
	inst.is.Status = pb.OInstanceState_PrePrepared
	pm := &pb.OrderMessage{
		View:                m.View,
		Index:               m.Index,
		Type:                pb.OrderMessage_Prepare,
		POSummaryMatrixHash: m.POSummaryMatrixHash,
	}
	inst.pCert.log(p.id, pm)
	p.broadcast(pm, false)
	if inst.pCert.Majority(pm) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.OInstanceState_Prepared
		cm := &pb.OrderMessage{
			View:                pm.View,
			Index:               pm.Index,
			Type:                pb.OrderMessage_Commit,
			POSummaryMatrixHash: pm.POSummaryMatrixHash,
		}
		p.broadcast(cm, false)

		inst.cCert.log(p.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.OInstanceState_Committed
			p.exec()
		}
	}
}

func (p *prime) onPrepare(m *pb.OrderMessage, from peerpb.PeerID) {
	if p.isCurrentPrimary(from) {
		p.logger.Errorf("Prepare received vcSender Primary %v: %v", from, m)
		return
	}

	inst, exists := p.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, p)
		p.olog[m.Index] = inst
	} else if !p.updateOCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.OInstanceState_Prepared
		cm := &pb.OrderMessage{
			View:                m.View,
			Index:               m.Index,
			Type:                pb.OrderMessage_Commit,
			POSummaryMatrixHash: m.POSummaryMatrixHash,
		}
		p.broadcast(cm, false)

		inst.cCert.log(p.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.OInstanceState_Committed
			p.exec()
		}
	}
}

// HandleP2b handles P2b message
func (p *prime) onCommit(m *pb.OrderMessage, from peerpb.PeerID) {
	inst, exists := p.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, p)
		p.olog[m.Index] = inst
	} else if !p.updateOCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m)
	if inst.cCert.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.OInstanceState_Committed
		p.exec()
	} else if inst.is.IsCommitted() {
		p.exec()
	}
	//p.logger.Debugf("commit acks for %v: %v", m.Command.ID, len(p.log[m.Index].cCert.msgs))
}

func (p *prime) isCurrentPrimary(id peerpb.PeerID) bool {
	return p.isPrimaryAtView(id, p.oview)
}

func (p *prime) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(p.nodes))
}
