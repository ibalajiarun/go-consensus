package threshsign2

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/cloudflare/bn256"
)

func bigFromBase10(s string) *big.Int {
	n, success := new(big.Int).SetString(s, 10)
	if !success {
		panic("failed to parse")
	}
	return n
}

var G2One = &bn256.G2{}

func init() {
	G2One.Unmarshal(bytes.Join([][]byte{
		{0x01},
		bigFromBase10("10857046999023057135944570762232829481370756359578518086990519993285655852781").Bytes(),
		bigFromBase10("11559732032986387107991004021392285783925812861821192530917403151452391805634").Bytes(),
		bigFromBase10("8495653923123431417604973247489272438418190587263600148770280649306958101930").Bytes(),
		bigFromBase10("4082367875863433681332203403145435568316851327593401208105741076214120093531").Bytes(),
	}, nil))
}

type pubKey struct {
	g2 *bn256.G2
}

func newPubKey(keys []string) *pubKey {
	pubKeyG2 := new(bn256.G2)
	pubKeyG2.Unmarshal(bytes.Join([][]byte{
		{0x01},
		bigFromBase10(keys[0]).Bytes(),
		bigFromBase10(keys[1]).Bytes(),
		bigFromBase10(keys[2]).Bytes(),
		bigFromBase10(keys[3]).Bytes(),
	}, nil))
	return &pubKey{
		g2: pubKeyG2,
	}
}

func (pk *pubKey) verify(hash [32]byte, ctr uint64, sig *signature) bool {
	for i := uint64(0); i < 4; i++ {
		hash[31-i] = hash[31-i] ^ uint8((ctr>>(8*i))&0xFF)
	}

	// hashG1 := bn256.HashG1(hash[:], nil)
	_, hashG1, _ := bn256.RandomG1(rand.Reader)
	pair1 := bn256.Pair(sig.g1, G2One)
	pair2 := bn256.Pair(hashG1, pk.g2)

	return *pair1 == *pair2
}

type signature struct {
	g1 *bn256.G1
}

func newSignature(s string) *signature {
	sigStrSlices := strings.Split(s, ":")
	sigG1 := &bn256.G1{}
	sigG1Bytes := append(bigFromBase10(sigStrSlices[0]).Bytes(), bigFromBase10(sigStrSlices[1]).Bytes()...)
	sigG1.Unmarshal(sigG1Bytes)
	return &signature{
		g1: sigG1,
	}
}
