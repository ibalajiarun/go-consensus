package chainhotstuff

import (
	"crypto"
	"crypto/rand"
	"testing"

	"github.com/oasisprotocol/ed25519"
)

func TestEd25519Keys(t *testing.T) {
	msg := []byte("hotstuffmessage")
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Unable to generate ed25519 keypair: %v", err)
	}
	sig, err := privKey.Sign(rand.Reader, msg, crypto.Hash(0))
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pubKey, msg, sig) {
		t.Fatal("unable to verify")
	}
}
