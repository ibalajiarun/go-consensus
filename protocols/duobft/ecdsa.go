package duobft

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"github.com/ibalajiarun/go-consensus/enclaves/usig"
)

const (
	privateKey = "MHcCAQEEIAA0R4fcx9fa7VD0EtHWcsnXwds8x6vI8WnDueH+YPY+oAoGCCqGSM49AwEHoUQDQgAEuebCi0tHwx1yDsJ1UbcjfpTkb+4e8oyIP7VqvMdswY3MWcHiWhCZzXAkET78a+dUIIy4W1qEcsN26RRRKcSt8w=="
	publicKey  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEuebCi0tHwx1yDsJ1UbcjfpTkb+4e8oyIP7VqvMdswY3MWcHiWhCZzXAkET78a+dUIIy4W1qEcsN26RRRKcSt8w=="
)

// parsePrivateKey parse the ECDSA private key in ASN.1 format encoded in base64
func parsePrivateKey(privKeyStr string) (*ecdsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(privKeyStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error (ECDSA private key): %v", err)
	}
	privKey, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse error (ECDSA private key): %v", err)
	}
	return privKey, nil
}

// parsePublicKey parse a DER encoded ECDSA public key in base64
func parsePublicKey(pubKeyStr string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(pubKeyStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error (ECDSA public key): %v", err)
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse error (ECDSA public Key): %v", err)
	}
	pubKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key format error: expect ECDSA")
	}
	return pubKey, nil
}

func (p *duobft) usigSign(msg []byte) []byte {
	mHash := sha256.Sum256(msg)
	ctr, sig := p.certifier.CreateUISig(mHash)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	return append(sig, ctrBytes...)
}

func (p *duobft) usigVerify(mHash usig.Digest, sig []byte) bool {
	ctrBytes := sig[len(sig)-8:]
	ctr := binary.LittleEndian.Uint64(ctrBytes)

	return p.certifier.VerifyUISig(mHash, sig[:len(sig)-8], ctr)
}

func (p *duobft) normalSign(msg []byte) []byte {
	sig, err := p.signer.Sign(msg)
	if err != nil {
		p.logger.Panic(err)
	}

	return sig
}

func (p *duobft) normalVerify(msg []byte, sig []byte) bool {
	return p.signer.Verify(msg, sig) != nil
}
