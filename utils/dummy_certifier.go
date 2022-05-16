package utils

type dummyCertifier struct {
}

func NewDummyCertifier() Certifier {
	return &dummyCertifier{}
}

func (dummyCertifier) Init() {

}

func (dummyCertifier) Destroy() {

}

func (c *dummyCertifier) CreateSignedCertificate(message []byte, tc interface{}) []byte {
	return []byte{0}
}

func (c *dummyCertifier) VerifyCertificate(message []byte, tc interface{}, mac []byte) bool {
	return true
}
