package utils

type Certifier interface {
	Init()
	Destroy()

	CreateSignedCertificate([]byte, interface{}) []byte
	VerifyCertificate([]byte, interface{}, []byte) bool
}
