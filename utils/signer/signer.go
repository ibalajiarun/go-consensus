package signer

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"math/big"
)

const (
	PRIVATE_KEY = "MHcCAQEEIAA0R4fcx9fa7VD0EtHWcsnXwds8x6vI8WnDueH+YPY+oAoGCCqGSM49AwEHoUQDQgAEuebCi0tHwx1yDsJ1UbcjfpTkb+4e8oyIP7VqvMdswY3MWcHiWhCZzXAkET78a+dUIIy4W1qEcsN26RRRKcSt8w=="
	PUBLIC_KEY = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEuebCi0tHwx1yDsJ1UbcjfpTkb+4e8oyIP7VqvMdswY3MWcHiWhCZzXAkET78a+dUIIy4W1qEcsN26RRRKcSt8w=="
)

type Signer struct {
	HashScheme crypto.Hash
	SigCipher  SignatureCipher
	privKey    interface{}
	pubKey     interface{}
}

func NewSigner() *Signer {
	keyspec := &ecdsaKeySpec{}
	pk, err := keyspec.parsePrivateKey(PRIVATE_KEY)
	if err != nil {
		panic(err)
	}
	puk, err := keyspec.parsePublicKey(PUBLIC_KEY)
	if err != nil {
		panic(err)
	}
	return &Signer{
		HashScheme: crypto.SHA256,
		SigCipher:  &EcdsaSigCipher{},
		privKey: pk,
		pubKey: puk,
	}
}

func (s *Signer) Sign(msg []byte) ([]byte, error) {
	md := s.HashScheme.New().Sum(msg)
	return s.SigCipher.Sign(md, s.privKey)
}

func (s *Signer) Verify(msg, sig []byte) error {
	md := s.HashScheme.New().Sum(msg)
	if !s.SigCipher.Verify(md, sig, s.pubKey) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// SignatureCipher defines the interface of signature operations used by public cryptographic ciphers
type SignatureCipher interface {
	// Sign creates signature over the message digest
	Sign(md []byte, privKey interface{}) ([]byte, error)
	// Verify verifies the signature over the message digest
	Verify(md, sig []byte, pubKey interface{}) bool
}

//========= SignatureCipher implementations =======

type (
	// EcdsaNIST256pSigCipher implements the SignatureCipher interface with
	// signature scheme EcdsaNIST256p
	EcdsaNIST256pSigCipher struct{}
)

// EcdsaSigCipher is alias to EcdsaNIST256pSigCipher
type EcdsaSigCipher EcdsaNIST256pSigCipher

// type assertions for interface impl.
var _ = SignatureCipher(&EcdsaSigCipher{})

// ecdsaSignature gives the ASN.1 encoding of the signature
type ecdsaSignature struct {
	R, S *big.Int
}

// Sign returns an ECDSA signature that is encoded as ASN.1 der format
func (c *EcdsaSigCipher) Sign(md []byte, privKey interface{}) ([]byte, error) {
	if eccPrivKey, ok := privKey.(*ecdsa.PrivateKey); ok {
		r, s, err := ecdsa.Sign(rand.Reader, eccPrivKey, md)
		if err != nil {
			return nil, fmt.Errorf("ECDSA signing error: %v", err)
		}
		sig, err := asn1.Marshal(ecdsaSignature{r, s})
		if err != nil {
			return nil, fmt.Errorf("ECDSA signature ASN1-DER marshal error: %v", err)
		}
		return sig, nil
	}
	return nil, fmt.Errorf("incompatible format of ECDSA private key")
}

// Verify verifies a ECDSA signature that is encoded as ASN.1 der format
func (c *EcdsaSigCipher) Verify(md, sig []byte, pubKey interface{}) bool {
	ecdsaSig := &ecdsaSignature{}
	_, err := asn1.Unmarshal(sig, ecdsaSig)
	if err != nil {
		panic(fmt.Sprintf("ECDSA signature is not ASN.1-DER encoded: %v", err))
	}
	if ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey); ok {
		return ecdsa.Verify(ecdsaPubKey, md, ecdsaSig.R, ecdsaSig.S)
	}
	return false
}

//###### ECDSA #######

type ecdsaKeySpec struct{}

// parsePrivateKey parse the ECDSA private key in ASN.1 format encoded in base64
func (spec *ecdsaKeySpec) parsePrivateKey(privKeyStr string) (interface{}, error) {
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
func (spec *ecdsaKeySpec) parsePublicKey(pubKeyStr string) (interface{}, error) {
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

func (spec *ecdsaKeySpec) generateKeyPair(securityParam int) (string, string, error) {
	var curve elliptic.Curve
	switch securityParam {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		// same curve used by SGX
		curve = elliptic.P256()
	}
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return "", "", err
	}
	privKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		panic(fmt.Sprintf("x509.MarshalECPrivateKey failed: %v", err))
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(fmt.Sprintf("x509.MarshalPKIXPublicKey failed: %v", err))
	}

	privKeyBase64 := base64.StdEncoding.EncodeToString(privKeyBytes)
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

	return privKeyBase64, pubKeyBase64, nil
}

