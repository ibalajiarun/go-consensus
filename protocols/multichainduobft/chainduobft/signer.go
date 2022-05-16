package chainduobft

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"github.com/ibalajiarun/go-consensus/enclaves/usig"
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/multichainduobft/chainduobft/chainduobftpb"
	"github.com/oasisprotocol/ed25519"
)

// ErrHashMismatch is the error used when a partial certificate hash does not match the hash of a block.
var ErrHashMismatch = fmt.Errorf("certificate hash does not match block hash")

// ErrPartialDuplicate is the error used when two or more signatures were created by the same replica.
var ErrPartialDuplicate = fmt.Errorf("cannot add more than one signature per replica")

type signer struct {
	ctrID     int
	enclave   *usig.UsigEnclave
	privKey   ed25519.PrivateKey
	publicKey ed25519.PublicKey
}

func newSigner(ctrID int, enclave *usig.UsigEnclave, privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) *signer {
	return &signer{
		ctrID:     ctrID,
		enclave:   enclave,
		privKey:   privKey,
		publicKey: pubKey,
	}
}

func (s *signer) sign(hash [sha512.Size]byte) ([]byte, error) {
	ctrBytes := make([]byte, 12)
	binary.LittleEndian.PutUint32(ctrBytes, uint32(s.ctrID))

	ctrMsg := append(ctrBytes, hash[:sha256.Size]...)

	sig, err := s.privKey.Sign(rand.Reader, ctrMsg, crypto.Hash(0))
	if err != nil {
		return nil, err
	}
	ctrsig := append(ctrBytes[4:], sig...)

	return ctrsig, nil
}

func (s *signer) signUsig(hash [sha512.Size]byte) ([]byte, error) {
	ctr, sig := s.enclave.CreateUISigEd25519(s.ctrID, usig.Digest(*(*[sha256.Size]byte)(hash[:sha256.Size])))

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)
	ctrsig := append(ctrBytes, sig...)

	return ctrsig, nil
}

func (s *signer) verify(msg [sha512.Size]byte, sig *pb.Signature) bool {
	sigBytes := sig.Sig
	ctr := sig.Counter

	ctrBytes := make([]byte, 12)
	binary.LittleEndian.PutUint32(ctrBytes, uint32(s.ctrID))
	binary.LittleEndian.PutUint64(ctrBytes[4:], ctr)
	ctrMsg := append(ctrBytes, msg[:sha256.Size]...)

	return ed25519.VerifyWithOptions(s.publicKey, ctrMsg, sigBytes, &ed25519.Options{Hash: crypto.Hash(0)})
}

func (s *signer) createQuorumCert(hash pb.BlockHashSlice, height pb.Height, signatures []pb.VoteMessage) (cert *pb.QuorumCert, err error) {
	qc := &pb.QuorumCert{
		Sigs:      make(map[peerpb.PeerID]*pb.Signature),
		BlockHash: hash,
		Height:    height,
	}
	for _, s := range signatures {
		blockHash := s.BlockHash
		if !bytes.Equal(hash, blockHash) {
			return nil, ErrHashMismatch
		}
		if _, ok := qc.Sigs[s.Signature.Signer]; ok {
			return nil, ErrPartialDuplicate
		}
		qc.Sigs[s.Signature.Signer] = s.Signature
	}
	return qc, nil
}

func (s *signer) verifyQuorumCert(qc *pb.QuorumCert, quorumSize int) bool {
	// If QC was created for genesis, then skip verification.
	if qc.BlockHash.Array() == genesis.hash() {
		return true
	}

	if len(qc.Sigs) < quorumSize {
		return false
	}
	pubKeys := make([]ed25519.PublicKey, len(qc.Sigs))
	msgs := make([][]byte, len(qc.Sigs))
	sigs := make([][]byte, len(qc.Sigs))

	ctrBytes := make([]byte, 12)
	var ctrMsg []byte
	i := 0
	for _, sig := range qc.Sigs {
		if ctrMsg == nil {
			binary.LittleEndian.PutUint32(ctrBytes, uint32(s.ctrID))
			binary.LittleEndian.PutUint64(ctrBytes[4:], sig.Counter)
			ctrMsg = append(ctrBytes, qc.BlockHash[:sha256.Size]...)
		}
		msgs[i] = ctrMsg
		sigs[i] = sig.Sig
		pubKeys[i] = s.publicKey
		i++
	}
	allOk, ok, err := ed25519.VerifyBatch(rand.Reader, pubKeys, msgs, sigs, &ed25519.Options{Hash: crypto.Hash(0)})
	if err != nil {
		panic(err)
	}
	if allOk {
		return allOk
	}
	cnt := 0
	for i := range ok {
		if ok[i] {
			cnt++
		}
	}
	return cnt >= quorumSize
}
