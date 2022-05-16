package minbft

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"

	"github.com/gogo/protobuf/proto"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/minbft/minbftpb"
)

func (p *minbft) wrapAndMarshal(m proto.Message) []byte {
	hMsg := pb.WrapMinBFTMessage(m)

	mBytes, err := proto.Marshal(hMsg)
	if err != nil {
		p.logger.Panic("unable to marshal")
	}
	return mBytes
}

func (p *minbft) marshalAndSignNormal(m *pb.NormalMessage) ([]byte, []byte) {
	mBytes := p.wrapAndMarshal(m)
	mHash := sha256.Sum256(mBytes)
	counter, cert := p.certifier.CreateUIMac(mHash)

	uiBuffer := bytes.NewBuffer(cert)
	binary.Write(uiBuffer, binary.LittleEndian, counter)

	return mBytes, uiBuffer.Bytes()
}

func (p *minbft) sendTo(to peerpb.PeerID, mm, cert []byte) {
	cm := peerpb.Message{
		Content:     mm,
		Certificate: cert,
		To:          to,
		From:        p.id,
	}
	p.msgs = append(p.msgs, cm)
}

func (p *minbft) broadcastNormal(m *pb.NormalMessage) {
	mBytes, cert := p.marshalAndSignNormal(m)

	p.broadcast(mBytes, cert)
}

func (p *minbft) broadcast(mBytes, cert []byte) {
	for _, node := range p.nodes {
		if node != p.id {
			p.sendTo(node, mBytes, cert)
		}
	}
}

func (p *minbft) ClearMsgs() {
	p.msgs = nil
}
