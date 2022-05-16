package chainhotstuff

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	pb "github.com/ibalajiarun/go-consensus/protocols/chainhotstuff/chainhotstuffpb"
	"github.com/oasisprotocol/ed25519"
)

// ErrHashMismatch is the error used when a partial certificate hash does not match the hash of a block.
var ErrHashMismatch = fmt.Errorf("certificate hash does not match block hash")

// ErrPartialDuplicate is the error used when two or more signatures were created by the same replica.
var ErrPartialDuplicate = fmt.Errorf("cannot add more than one signature per replica")

const (
	publicKeyHex  = "7454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
	privateKeyHex = "54ba20a8df7259de04c5a0396be3d0796ef1753d6515e9c9d9aea1929632a5b87454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
)

type signer struct {
	privKey    ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	quorumSize int
}

func newSigner(quorumSize int) *signer {
	pubKey, _ := hex.DecodeString(publicKeyHex)
	privKey, _ := hex.DecodeString(privateKeyHex)
	return &signer{
		quorumSize: quorumSize,
		privKey:    privKey,
		pubKey:     pubKey,
	}
}

func (s *signer) sign(hash [sha512.Size]byte) ([]byte, error) {
	sig, err := s.privKey.Sign(rand.Reader, hash[:], crypto.SHA512)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func (s *signer) verify(msg [sha512.Size]byte, sig []byte) bool {
	return ed25519.VerifyWithOptions(s.pubKey, msg[:], sig, &ed25519.Options{Hash: crypto.SHA512})
}

func (s *signer) createQuorumCert(block *block, signatures []pb.VoteMessage) (cert *pb.QuorumCert, err error) {
	hash := block.hash()
	qc := &pb.QuorumCert{
		Sigs:      make(map[peerpb.PeerID]*pb.Signature),
		BlockHash: hash[:],
	}
	for _, s := range signatures {
		blockHash := s.BlockHash
		if !bytes.Equal(hash[:], blockHash) {
			return nil, ErrHashMismatch
		}
		if _, ok := qc.Sigs[s.Signature.Signer]; ok {
			return nil, ErrPartialDuplicate
		}
		qc.Sigs[s.Signature.Signer] = s.Signature
	}
	return qc, nil
}

func (s *signer) verifyQuorumCert(qc *pb.QuorumCert) bool {
	// If QC was created for genesis, then skip verification.
	if qc.BlockHash.Array() == genesis.hash() {
		return true
	}

	if len(qc.Sigs) < s.quorumSize {
		return false
	}
	pubKeys := make([]ed25519.PublicKey, len(qc.Sigs))
	msgs := make([][]byte, len(qc.Sigs))
	sigs := make([][]byte, len(qc.Sigs))
	i := 0
	for _, sig := range qc.Sigs {
		msgs[i] = qc.BlockHash
		sigs[i] = sig.Sig
		pubKeys[i] = s.pubKey
		i++
	}
	allOk, ok, err := ed25519.VerifyBatch(rand.Reader, pubKeys, msgs, sigs, &ed25519.Options{Hash: crypto.SHA512})
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
	return cnt >= s.quorumSize
}
