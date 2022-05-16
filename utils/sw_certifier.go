package utils

import (
	"crypto/hmac"
	"crypto/sha256"
)

var key = []byte("thisisasamplekeyforswmaccreation")

type swCertifier struct {
}

func NewSWCertifier() Certifier {
	return &swCertifier{}
}

func (*swCertifier) Init() {

}

func (*swCertifier) Destroy() {

}

func (c *swCertifier) CreateSignedCertificate(message []byte, tc interface{}) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(message)
	return hash.Sum(nil)
}

func (c *swCertifier) VerifyCertificate(message []byte, tc interface{}, mac []byte) bool {
	hash := hmac.New(sha256.New, key)
	hash.Write(message)
	return hmac.Equal(hash.Sum(nil), mac)
}
