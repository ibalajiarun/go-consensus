package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

func BenchmarkMacComputation(t *testing.B) {
	mac := hmac.New(sha256.New, []byte("thisisasamplekeyforswmaccreation"))
	message := make([]byte, 2048)
	rand.Read(message)

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		mac.Reset()
		_ = mac.Sum(message)
	}
}

func TestSWCertifier(t *testing.T) {
	signer := NewSWCertifier()

	var msg [256]byte
	rand.Read(msg[:])

	cert := signer.CreateSignedCertificate(msg[:], 0)
	if !signer.VerifyCertificate(msg[:], 0, cert) {
		t.Fatalf("Invalid certificate")
	}

	if signer.VerifyCertificate(msg[:10], 0, cert) {
		t.Fatalf("Invalid certificate")
	}

	if len(cert) != 32 {
		t.Fatalf("invalid mac size: %d", len(cert))
	}
}
