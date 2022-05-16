package usig

/*
#include "enclave_usig/untrusted/usig.h"
#cgo LDFLAGS: -L${SRCDIR}/enclave_usig -lusig
#cgo CFLAGS: -I/opt/sgxsdk/include
*/
import "C"
import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/pkg/errors"
)

const enclaveFileName = "usig_enclave.signed.so"

type Digest [sha256.Size]byte

type UI struct {
	counter     uint64
	certificate []byte
}

type UsigEnclave struct {
	id          uint64
	enclaveFile string
	pubKey      ecdsa.PublicKey
}

func NewUsigEnclave(filepath string) *UsigEnclave {
	return &UsigEnclave{
		enclaveFile: fmt.Sprintf("%s/%s", filepath, enclaveFileName),
	}
}

func (e *UsigEnclave) Init(mackey []byte, sigkey *ecdsa.PrivateKey, edprivkey []byte, numCounters int) {
	var eid uint64
	cEnclavePath := C.CString(e.enclaveFile)
	sgxSigKey := C.sgx_ec256_private_t{
		r: sgxBigIntToUint8Slice(sigkey.D),
	}

	ret := C.usig_init(
		cEnclavePath,
		(*C.char)(unsafe.Pointer(&mackey[0])), C.uint32_t(len(mackey)),
		sgxSigKey,
		C.uint32_t(numCounters),
		(*C.char)(unsafe.Pointer(&edprivkey[0])), C.uint32_t(len(edprivkey)),
		(*C.uint64_t)(unsafe.Pointer(&eid)))
	if ret != 0 {
		panic(errors.Errorf("unable to initialize enclave. return code: %x", ret))
	}
	e.id = eid
	e.pubKey = sigkey.PublicKey
	C.free(unsafe.Pointer(cEnclavePath))
}

func (e *UsigEnclave) Destroy() {
	C.usig_destroy(C.uint64_t(e.id))
}

func (e *UsigEnclave) CreateUISig(digest Digest) (counter uint64, signature []byte) {
	sgxSignature := new(C.sgx_ec256_signature_t)

	ret := C.usig_create_ui_sig(
		C.uint64_t(e.id),
		(*C.uint8_t)(&digest[0]),
		(*C.uint64_t)(&counter),
		sgxSignature)
	if ret != 0 {
		panic(fmt.Sprintf("Unable to create independent counter certificate %x\n", ret))
	}

	return counter, sgxEC256SigToASN1(sgxSignature)
}

func (e *UsigEnclave) CreateUIMac(digest Digest) (counter uint64, signature []byte) {
	hmac := make([]byte, 32)
	ret := C.usig_create_ui_mac(
		C.uint64_t(e.id),
		(*C.uint8_t)(&digest[0]),
		(*C.uint64_t)(&counter),
		(*C.uint8_t)(unsafe.Pointer(&hmac[0])),
		C.uint32_t(len(hmac)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to create continuing counter certificate %x", ret))
	}

	return counter, hmac
}

func (e *UsigEnclave) VerifyUISig(digest Digest, sig []byte, ctr uint64) bool {
	buf := bytes.NewBuffer(digest[:])
	err := binary.Write(buf, binary.LittleEndian, ctr)
	if err != nil {
		panic(err)
	}

	hash := sha256.Sum256(buf.Bytes())

	return ecdsa.VerifyASN1(&e.pubKey, hash[:], sig)
}

func (e *UsigEnclave) VerifyUIMac(digest Digest, mac []byte, counter uint64) bool {
	identical := false
	ret := C.usig_verify_ui_mac(
		C.uint64_t(e.id),
		(*C.uint8_t)(&digest[0]),
		(C.uint64_t)(counter),
		(*C.uint8_t)(unsafe.Pointer(&mac[0])),
		C.uint32_t(len(mac)),
		(*C.uint8_t)(unsafe.Pointer(&identical)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to verify continuing counter certificate %v", ret))
	}

	return identical
}

func sgxEC256SigToASN1(sgxSig *C.sgx_ec256_signature_t) []byte {
	sgxR, sgxS := sgxSig.x[:], sgxSig.y[:]
	r := sgxUint32SliceToBigInt(sgxR)
	s := sgxUint32SliceToBigInt(sgxS)

	ret, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		panic(err)
	}

	return ret
}

func sgxUint32SliceToBigInt(sgxArr []C.uint32_t) *big.Int {
	bytes := make([]byte, 0, len(sgxArr)*4)

	// Convert SGX representation encoded as a series of 32-bit
	// integers in little endian order to a big endian byte slice
	for i := len(sgxArr); i > 0; i-- {
		w := sgxArr[i-1]
		bytes = append(bytes,
			byte(w>>(3*8)),
			byte(w>>(2*8)),
			byte(w>>(1*8)),
			byte(w>>(0*8)))
	}

	return new(big.Int).SetBytes(bytes)
}

func sgxBigIntToUint8Slice(bigInt *big.Int) [32]C.uint8_t {
	bytes := bigInt.Bytes()
	bytesLen := len(bytes)
	var sgxArr [32]C.uint8_t

	// Convert SGX representation encoded as a series of bytes in
	// little endian order to big endian byte slice
	for i := bytesLen; i > 0; i-- {
		sgxArr[i-1] = C.uint8_t(bytes[bytesLen-i])
	}

	return sgxArr
}

func (e *UsigEnclave) CreateUISigEd25519(ctrID int, digest Digest) (uint64, []byte) {
	sgxSignature := make([]byte, 64)
	var counter uint64

	ret := C.usig_create_ui_sig_edd25519(
		C.uint64_t(e.id),
		(*C.uint8_t)(&digest[0]),
		C.uint32_t(ctrID),
		(*C.uint64_t)(&counter),
		(*C.uint8_t)(unsafe.Pointer(&sgxSignature[0])),
		C.uint32_t(len(sgxSignature)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to create independent counter certificate %x\n", ret))
	}

	return counter, sgxSignature
}
