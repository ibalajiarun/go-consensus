package usig

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/oasisprotocol/ed25519"
)

func TestUSIGMac(t *testing.T) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, []byte{0}, 1)
	ctr, cert := sgx.CreateUIMac(mHash)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	goMac := hmac.New(sha256.New, mackey)
	goMac.Write(mHash[:])
	goMac.Write(ctrBytes)
	if expected := goMac.Sum(nil); !bytes.Equal(cert, expected) {
		t.Errorf("hmac mismatch: (expected) %x(%d) != (actual) %x(%d)", expected, len(expected), cert, len(cert))
	}

	if !sgx.VerifyUIMac(mHash, cert, ctr) {
		t.Error("could not verify certificate")
	}

	sgx.Destroy()
}

func TestUSIGSig(t *testing.T) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, []byte{0}, 1)
	ctr, cert := sgx.CreateUISig(mHash)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	if !sgx.VerifyUISig(mHash, cert, ctr) {
		t.Error("could not verify certificate")
	}

	sgx.Destroy()
}

func BenchmarkUSIGSign(b *testing.B) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, []byte{0}, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sgx.CreateUISig(mHash)
	}
	b.StopTimer()

	sgx.Destroy()
}

func BenchmarkSign(b *testing.B) {
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ecdsa.SignASN1(rand.Reader, pkey, mHash[:])
		if err != nil {
			panic(err)
		}
	}
	b.StopTimer()
}

func BenchmarkUSIGSVerify(b *testing.B) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, []byte{0}, 1)
	ctr, cert := sgx.CreateUISig(mHash)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !sgx.VerifyUISig(mHash, cert, ctr) {
			b.Error("could not verify certificate")
		}
	}
	b.StopTimer()

	sgx.Destroy()
}

func BenchmarkUSIGSVerifyParallel(b *testing.B) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, []byte{0}, 1)
	ctr, cert := sgx.CreateUISig(mHash)

	ctrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ctrBytes, ctr)

	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			if !sgx.VerifyUISig(mHash, cert, ctr) {
				b.Error("could not verify certificate")
			}
		}
	})
	b.StopTimer()

	sgx.Destroy()
}

const (
	edPublicKeyHex  = "7454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
	edPrivateKeyHex = "54ba20a8df7259de04c5a0396be3d0796ef1753d6515e9c9d9aea1929632a5b87454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
)

func TestUSIGSigEd25519(t *testing.T) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edPubKey, _ := hex.DecodeString(edPublicKeyHex)
	edPrivKey, _ := hex.DecodeString(edPrivateKeyHex)

	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	// cidBytes := make([]byte, 4)
	// binary.LittleEndian.PutUint32(cidBytes, 1)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, edPrivKey, 1)
	ctr, sig := sgx.CreateUISigEd25519(0, mHash)

	ctrBytes := make([]byte, 12)
	binary.LittleEndian.PutUint64(ctrBytes, 0)
	binary.LittleEndian.PutUint64(ctrBytes[4:], ctr)
	msg = append(ctrBytes, mHash[:]...)

	if !ed25519.Verify(edPubKey, msg[:], sig) {
		t.Error("could not verify signature")
	}

	sgx.Destroy()
}

func TestUSIGSigEd25519MultiCounters(t *testing.T) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edPubKey, _ := hex.DecodeString(edPublicKeyHex)
	edPrivKey, _ := hex.DecodeString(edPrivateKeyHex)

	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, edPrivKey, 10)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			ctr, sig := sgx.CreateUISigEd25519(i, mHash)

			ctrBytes := make([]byte, 12)
			binary.LittleEndian.PutUint64(ctrBytes, uint64(i))
			binary.LittleEndian.PutUint64(ctrBytes[4:], ctr)
			msg = append(ctrBytes, mHash[:]...)

			if !ed25519.Verify(edPubKey, msg[:], sig) {
				t.Fatal("could not verify signature")
			}
		}
	}
	sgx.Destroy()

}

func BenchmarkUSIGSigEd25519Sign(b *testing.B) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	edPrivKey, _ := hex.DecodeString(edPrivateKeyHex)

	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, edPrivKey, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sgx.CreateUISigEd25519(0, mHash)
	}
	b.StopTimer()

	sgx.Destroy()
}

func BenchmarkUSIGSigEd25519Verify(b *testing.B) {
	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	edPubKey, _ := hex.DecodeString(edPublicKeyHex)
	edPrivKey, _ := hex.DecodeString(edPrivateKeyHex)

	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, edPrivKey, 1)
	ctr, sig := sgx.CreateUISigEd25519(0, mHash)

	ctrBytes := make([]byte, 12)
	binary.LittleEndian.PutUint64(ctrBytes, 0)
	binary.LittleEndian.PutUint64(ctrBytes[4:], ctr)
	msg = append(ctrBytes, mHash[:]...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !ed25519.Verify(edPubKey, msg[:], sig) {
			b.Error("could not verify signature")
		}
	}
	b.StopTimer()

	sgx.Destroy()
}

func BenchmarkUSIGSigEd25519VerifyBatch(b *testing.B) {
	length := 33

	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	edPubKey, _ := hex.DecodeString(edPublicKeyHex)
	edPrivKey, _ := hex.DecodeString(edPrivateKeyHex)

	msg := []byte("testmessage")
	mHash := sha256.Sum256(msg)

	sgx := NewUsigEnclave("/home/balaji/workplace/mygo/go-consensus/enclaves/usig/enclave_usig/")
	sgx.Init(mackey, pkey, edPrivKey, 1)
	ctr, sig := sgx.CreateUISigEd25519(0, mHash)

	pubKeys := make([]ed25519.PublicKey, length)
	msgs := make([][]byte, length)
	sigs := make([][]byte, length)

	ctrBytes := make([]byte, 12)
	var ctrMsg []byte
	for i := 0; i < length; i++ {
		if ctrMsg == nil {
			binary.LittleEndian.PutUint32(ctrBytes, 0)
			binary.LittleEndian.PutUint64(ctrBytes[4:], ctr)
			ctrMsg = append(ctrBytes, mHash[:]...)
		}
		msgs[i] = ctrMsg
		sigs[i] = sig
		pubKeys[i] = edPubKey
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allOk, _, err := ed25519.VerifyBatch(rand.Reader, pubKeys, msgs, sigs, &ed25519.Options{Hash: crypto.Hash(0)})
		if err != nil || !allOk {
			panic(err)
		}
	}
	b.StopTimer()

	sgx.Destroy()
}
