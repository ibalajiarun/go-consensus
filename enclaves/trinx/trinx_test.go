package trinx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"testing"
)

func TestIndependentCertificate(t *testing.T) {
	msgSize := 1024
	key := []byte("thisisasamplekeyforenclave")
	msg := make([]byte, msgSize)
	if _, err := rand.Read(msg); err != nil {
		t.Fatal(err)
	}

	ctr := uint64(1)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	cidBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewTrInxEnclave("enclave_trinx/")
	sgx.Init(key, 3, 1)
	cert := sgx.CreateIndependentCounterCertificate(msg, ctr, 1)

	goMac := hmac.New(sha256.New, key)
	goMac.Write(cidBytes)
	goMac.Write(ctrBytes)
	goMac.Write(msg)
	if expected := goMac.Sum(nil); !bytes.Equal(cert, expected) {
		t.Errorf("hmac mismatch: (expected) %x(%d) != (actual) %x(%d)", expected, len(expected), cert, len(cert))
	}

	if !sgx.VerifyIndependentCounterCertificate(msg, cert, ctr, 1) {
		t.Error("could not verify certificate")
	}

	sgx.Destroy()
}

func TestIndependentCertificateVector(t *testing.T) {
	msgSize := 1024
	key := []byte("thisisasamplekeyforenclave")
	msg := make([]byte, msgSize)
	if _, err := rand.Read(msg); err != nil {
		t.Fatal(err)
	}

	ctr := uint64(1)
	ctrId := 0

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	cidBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(cidBytes, uint32(ctrId))

	sgx := NewTrInxEnclave("enclave_trinx/")
	sgx.Init(key, 1, 7)
	cert := sgx.CreateIndependentCounterCertificateVector(msg, ctr, ctrId)

	goMac := hmac.New(sha256.New, key)
	goMac.Write(cidBytes)
	goMac.Write(ctrBytes)
	goMac.Write(msg)
	if expected := goMac.Sum(nil); !bytes.Equal(cert[:32], expected) {
		t.Errorf("hmac mismatch: (expected) %x(%d) != (actual) %x(%d)", expected, len(expected), cert[:32], len(cert[:32]))
	}

	if !sgx.VerifyIndependentCounterCertificate(msg, cert[:32], ctr, ctrId) {
		t.Error("could not verify certificate")
	}

	sgx.Destroy()
}
