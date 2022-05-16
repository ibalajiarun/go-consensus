package dqpbft

import (
	"bytes"
	"crypto/sha256"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/dqpbft/dqpbftpb"
)

type oinstance struct {
	p *DQPBFT

	is pb.OInstanceState

	pCert *OQuorum
	cCert *OQuorum
}

func makeOInstance(m *pb.OAgreementMessage, p *DQPBFT) *oinstance {
	return &oinstance{
		is: pb.OInstanceState{
			View:        m.View,
			Index:       m.Index,
			Status:      pb.OInstanceState_None,
			Instances:   m.Instances,
			CommandHash: m.CommandHash,
		},
		pCert: newOQuorum(p),
		cCert: newOQuorum(p),
	}
}

func (d *DQPBFT) updateOCommand(inst *oinstance, m *pb.OAgreementMessage) bool {
	if inst.is.Instances != nil {
		if !pb.InstancesEquals(inst.is.Instances, m.Instances) {
			d.logger.Panicf("Different ocommand for same instance %v: Has: %v, Received %v.", inst.is, inst.is.Instances, m.Instances)
			return false
		}
	} else {
		inst.is.Instances = m.Instances
	}
	return true
}

func (d *DQPBFT) updateOCommandHash(inst *oinstance, m *pb.OAgreementMessage) bool {
	if inst.is.CommandHash != nil {
		if !bytes.Equal(inst.is.CommandHash, m.CommandHash) {
			d.logger.Panicf("Different command hashes for same instance %v: Received: %v, Has %v.", inst, inst.is.CommandHash, m.CommandHash)
			// TODO: Trigger view change here.
			return false
		}
	} else {
		inst.is.CommandHash = m.CommandHash
	}
	return true
}

func (d *DQPBFT) onCRequest(id pb.InstanceID) {
	d.logger.Debugf("Replica %v received CRequest(%v)\n", d.id, id)
	if !d.isPrimaryAtView(d.id, d.oview) {
		d.logger.Errorf("Not the leader of view %d: %d", d.oview, d.id)
		return
	}

	d.cReqBuffer = append(d.cReqBuffer, id)
	if len(d.cReqBuffer) >= int(d.oBatchSize) {
		d.sendCRequest()
	}
}

func (d *DQPBFT) sendCRequest() {
	if !d.isPrimaryAtView(d.id, d.oview) {
		d.logger.Errorf("Not the leader of view %d: %d", d.oview, d.id)
		return
	}

	ids := d.cReqBuffer
	d.cReqBuffer = nil

	if len(ids) <= 0 {
		return
	}

	d.logger.Debugf("sendCRequest: %v", ids)
	index := d.oindex.GetAndIncrement()
	inst := &oinstance{
		is: pb.OInstanceState{
			View:      d.view,
			Index:     index,
			Status:    pb.OInstanceState_OPrePrepared,
			Instances: ids,
		},
		pCert: newOQuorum(d),
		cCert: newOQuorum(d),
	}
	d.olog[index] = inst

	ppm := &pb.OAgreementMessage{
		View:      d.view,
		Index:     index,
		Type:      pb.OAgreementMessage_OPrePrepare,
		Instances: ids,
	}

	hasher := sha256.New()
	for _, instID := range ids {
		bytes, err := proto.Marshal(&instID)
		if err != nil {
			panic(err)
		}
		hasher.Write(bytes)
	}
	cmdHash := hasher.Sum(nil)
	logppm := &pb.OAgreementMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.OAgreementMessage_OPrePrepare,
		CommandHash: cmdHash,
	}
	inst.pCert.log(d.id, logppm)

	d.broadcast(ppm, false)
}

func (d *DQPBFT) onOPrePrepare(m *pb.OAgreementMessage, from peerpb.PeerID) {
	// Check if the message sender is the current primary
	if !d.isPrimaryAtView(from, d.oview) {
		d.logger.Panicf("PrePrepare vcSender Non-Primary %v: %v", from, m)
		return
	}

	hasher := sha256.New()
	for _, instID := range m.Instances {
		bytes, err := proto.Marshal(&instID)
		if err != nil {
			panic(err)
		}
		hasher.Write(bytes)
	}
	cmdHash := hasher.Sum(nil)
	m.CommandHash = cmdHash

	inst, exists := d.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, d)
		d.olog[m.Index] = inst
	} else if !d.updateOCommand(inst, m) || !d.updateOCommandHash(inst, m) {
		return
	}

	// The PrePrepare message of the primary serves as the prepare message.
	inst.pCert.log(from, m)
	inst.is.Status = pb.OInstanceState_OPrePrepared
	pm := &pb.OAgreementMessage{
		View:        inst.is.View,
		Index:       inst.is.Index,
		Type:        pb.OAgreementMessage_OPrepare,
		CommandHash: inst.is.CommandHash,
	}
	inst.pCert.log(d.id, pm)
	d.broadcast(pm, false)
	if inst.pCert.Majority(pm) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.OInstanceState_OPrepared
		cm := &pb.OAgreementMessage{
			View:        pm.View,
			Index:       pm.Index,
			Type:        pb.OAgreementMessage_OCommit,
			CommandHash: inst.is.CommandHash,
		}
		d.broadcast(cm, false)

		inst.cCert.log(d.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.OInstanceState_OCommitted
			d.exec()
		}
	}
}

func (d *DQPBFT) onOPrepare(m *pb.OAgreementMessage, from peerpb.PeerID) {
	if d.isPrimaryAtView(from, d.oview) {
		d.logger.Panicf("Prepare received vcSender Primary %v: %v", from, m)
		return
	}

	inst, exists := d.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, d)
		d.olog[m.Index] = inst
	} else if !d.updateOCommandHash(inst, m) {
		return
	}

	inst.pCert.log(from, m)
	if inst.pCert.Majority(m) && inst.is.IsPrePrepared() {
		inst.is.Status = pb.OInstanceState_OPrepared
		cm := &pb.OAgreementMessage{
			View:        m.View,
			Index:       m.Index,
			Type:        pb.OAgreementMessage_OCommit,
			CommandHash: inst.is.CommandHash,
		}
		d.broadcast(cm, false)

		inst.cCert.log(d.id, cm)
		if inst.cCert.Majority(cm) && inst.is.IsPrepared() {
			inst.is.Status = pb.OInstanceState_OCommitted
			d.exec()
		}
	}
}

// HandleP2b handles P2b message
func (d *DQPBFT) onOCommit(m *pb.OAgreementMessage, from peerpb.PeerID) {
	inst, exists := d.olog[m.Index]
	// Update the log with the message if it not known already. Otherwise,
	// ensure that the log is consistent with the message.
	if !exists {
		inst = makeOInstance(m, d)
		d.olog[m.Index] = inst
	} else if !d.updateOCommandHash(inst, m) {
		return
	}

	inst.cCert.log(from, m)
	if inst.cCert.Majority(m) && inst.is.IsPrepared() {
		inst.is.Status = pb.OInstanceState_OCommitted
		d.exec()
	}
	//p.logger.Debugf("commit acks for %v: %v", m.Command.ID, len(p.log[m.Index].cCert.msgs))
}

func (d *DQPBFT) isPrimaryAtView(id peerpb.PeerID, view pb.View) bool {
	return id == peerpb.PeerID(int(view)%len(d.peers))
}
