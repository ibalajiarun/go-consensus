package trinx

/*
#include "enclave_trinx/untrusted/TrInx.h"
#cgo LDFLAGS: -L${SRCDIR}/enclave_trinx -ltrinx
#cgo CFLAGS: -I/opt/sgxsdk/include
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
)

const enclaveFileName = "trinxenclave.signed.so"

type TrInxEnclave struct {
	id          uint64
	enclaveFile string
	vecLen      int
}

func NewTrInxEnclave(filepath string) *TrInxEnclave {
	return &TrInxEnclave{
		enclaveFile: fmt.Sprintf("%s/%s", filepath, enclaveFileName),
	}
}

func (e *TrInxEnclave) Init(key []byte, numCtrs int, vecLen int) {
	var eid uint64
	cEnclavePath := C.CString(e.enclaveFile)
	ret := C.trinx_init(
		cEnclavePath,
		(*C.char)(unsafe.Pointer(&key[0])), C.uint32_t(len(key)),
		C.uint32_t(numCtrs),
		C.uint32_t(vecLen),
		(*C.uint64_t)(unsafe.Pointer(&eid)))
	if ret != 0 {
		panic(errors.Errorf("unable to initialize enclave. return code: %x", ret))
	}
	e.id = eid
	e.vecLen = vecLen
	C.free(unsafe.Pointer(cEnclavePath))
}

func (e *TrInxEnclave) Destroy() {
	C.trinx_destroy(C.uint64_t(e.id))
}

func (e *TrInxEnclave) CreateIndependentCounterCertificate(msg []byte, tc uint64, ctrId int) []byte {
	hmac := make([]byte, 32)

	ret := C.trinx_create_independent_counter_certificate(
		C.uint64_t(e.id),
		C.uint32_t(ctrId),
		C.uint64_t(tc),
		(*C.uint8_t)(unsafe.Pointer(&msg[0])),
		C.uint32_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&hmac[0])),
		C.uint32_t(len(hmac)))
	if ret != 0 {
		bytesBuffer := bytes.NewBuffer(hmac[:8])
		var tmp uint64
		binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
		panic(fmt.Sprintf("Unable to create independent counter certificate %x: ctrId: %v, current: %v, received: %v\n", ret, ctrId, tmp, tc))
	}

	return hmac
}

func (e *TrInxEnclave) CreateIndependentCounterCertificateVector(msg []byte, tc uint64, ctrId int) []byte {
	hmac := make([]byte, 32*e.vecLen)

	ret := C.trinx_create_independent_counter_certificate_vector(
		C.uint64_t(e.id),
		C.uint32_t(ctrId),
		C.uint64_t(tc),
		(*C.uint8_t)(unsafe.Pointer(&msg[0])),
		C.uint32_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&hmac[0])),
		C.uint32_t(len(hmac)))
	if ret != 0 {
		bytesBuffer := bytes.NewBuffer(hmac[:8])
		var tmp uint64
		binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
		panic(fmt.Sprintf("Unable to create independent counter certificate %x: ctr: %v, current: %v, received: %v\n", ret, ctrId, tmp, tc))
	}

	return hmac
}

func (e *TrInxEnclave) CreateContinuingCounterCertificate(msg []byte, tc uint64, ctrId int) []byte {
	hmac := make([]byte, 32)
	ret := C.trinx_create_continuing_counter_certificate(
		C.uint64_t(e.id),
		C.uint32_t(ctrId),
		C.uint64_t(tc),
		(*C.uint8_t)(unsafe.Pointer(&msg[0])),
		C.uint32_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&hmac[0])),
		C.uint32_t(len(hmac)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to create continuing counter certificate %x", ret))
	}

	return hmac
}

func (e *TrInxEnclave) VerifyIndependentCounterCertificate(msg, mac []byte, tc uint64, ctrId int) bool {
	identical := false

	if len(mac) != 32 {
		panic(fmt.Sprintf("invalid certificate length: %v", len(mac)))
	}

	ret := C.trinx_verify_independent_counter_certificate(
		C.uint64_t(e.id),
		C.uint32_t(ctrId),
		C.uint64_t(tc),
		(*C.uint8_t)(unsafe.Pointer(&msg[0])),
		C.uint32_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&mac[0])),
		C.uint32_t(len(mac)),
		(*C.uint8_t)(unsafe.Pointer(&identical)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to verify independent counter certificate %v", ret))
	}
	return identical
}

func (e *TrInxEnclave) VerifyContinuingCounterCertificate(msg, mac []byte, oldCtr, newCtr uint64, ctrId int) bool {
	identical := false
	ret := C.trinx_verify_continuing_counter_certificate(
		C.uint64_t(e.id),
		C.uint32_t(ctrId),
		C.uint64_t(oldCtr),
		C.uint64_t(newCtr),
		(*C.uint8_t)(unsafe.Pointer(&msg[0])),
		C.uint32_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&mac[0])),
		C.uint32_t(len(mac)),
		(*C.uint8_t)(unsafe.Pointer(&identical)))
	if ret != 0 {
		panic(fmt.Sprintf("Unable to verify continuing counter certificate %v", ret))
	}

	return identical
}
