package chainduobft

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"

	"github.com/ibalajiarun/go-consensus/enclaves/usig"
	"github.com/oasisprotocol/ed25519"
)

const (
	publicKeyHex  = "7454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
	privateKeyHex = "54ba20a8df7259de04c5a0396be3d0796ef1753d6515e9c9d9aea1929632a5b87454ba59d3570463e92f76e8a17a9f38d0c222c31b4a40a625aee1c46d94efb6"
)

func MakeUsigSignerEnclave(enclaveFilePath string, numCounters int) (*usig.UsigEnclave, ed25519.PublicKey, ed25519.PrivateKey) {
	pubKey, _ := hex.DecodeString(publicKeyHex)
	privKey, _ := hex.DecodeString(privateKeyHex)

	mackey := []byte("thisisasamplekeyforenclave")
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("omg")
	}

	usigSigner := usig.NewUsigEnclave(enclaveFilePath)
	usigSigner.Init(mackey, pkey, privKey, numCounters)

	return usigSigner, pubKey, privKey
}
