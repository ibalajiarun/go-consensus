package threshsign

/*
#include "enclave_threshsign/untrusted/Threshsign.h"
#cgo LDFLAGS: -L${SRCDIR}/enclave_threshsign -lthreshsign
#cgo CFLAGS: -I/opt/sgxsdk/include
*/
import "C"

import (
	"crypto/sha256"
	"fmt"
	"unsafe"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/pkg/errors"
)

const enclaveFileName = "threshsignenclave.signed.so"

type ThreshsignEnclave struct {
	filepath  string
	threshold uint32
	total     uint32
	cPtr      *C.threshsign_t
}

func NewThreshsignEnclave(filepath string, threshold, total uint32) *ThreshsignEnclave {
	return &ThreshsignEnclave{
		filepath:  filepath,
		threshold: threshold,
		total:     total,
	}
}

func (s *ThreshsignEnclave) Init(index int, skey string, pkeys []string, numCounters uint32) {
	cPkeys := make([]*C.char, 0, len(pkeys))
	for _, k := range pkeys {
		cPkeys = append(cPkeys, C.CString(k))
	}

	cPkeyCombined := make([]byte, 512)
	cPkeyCombinedLen := len(cPkeyCombined)

	fmt.Println("Initializing SGX Enclave") //, pkt_bytes, len(pkt_bytes))

	cFilepath := C.CString(fmt.Sprintf("%s/%s", s.filepath, enclaveFileName))
	code := C.threshsign_setup(cFilepath,
		C.unsigned(len(s.filepath)),
		C.unsigned(s.threshold),
		C.unsigned(s.total),
		C.uint32_t(index),
		C.CString(skey),
		C.unsigned(len(skey)),
		(**C.char)(unsafe.Pointer(&cPkeys[0])),
		C.unsigned(len(pkeys)),
		(*C.char)(unsafe.Pointer(&cPkeyCombined[0])),
		(*C.unsigned)(unsafe.Pointer(&cPkeyCombinedLen)),
		C.uint32_t(numCounters),
		&s.cPtr)
	if code != 0 {
		panic(errors.Errorf("unable to initialize enclave. return code: %v", code))
	}

	for _, k := range cPkeys {
		C.free(unsafe.Pointer(k))
	}
	C.free(unsafe.Pointer(cFilepath))

	fmt.Println("Init done")
}

func (s *ThreshsignEnclave) Sign(message []byte, ctrId uint32, newValue uint64) []byte {
	signature := make([]byte, 256)
	sigLen := len(signature)
	hash := sha256.Sum256(message)

	ret := C.threshsign_sign(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.unsigned(len(hash)),
		C.uint32_t(ctrId),
		C.uint64_t(newValue),
		(*C.char)(unsafe.Pointer(&signature[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLen)))

	if ret != 0 {
		panic("Unable to sign")
	}
	return signature[:sigLen]
}

func (s *ThreshsignEnclave) SignUsingEnclave(message []byte, ctrId uint32, newValue uint64) []byte {
	signature := make([]byte, 256)
	sigLen := len(signature)
	hash := sha256.Sum256(message)

	ret := C.threshsign_sign_using_enclave(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.unsigned(len(hash)),
		C.uint32_t(ctrId),
		C.uint64_t(newValue),
		(*C.char)(unsafe.Pointer(&signature[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLen)))

	if ret != 0 {
		panic(fmt.Sprintf("Unable to sign using enclave: %x: %s", ret, signature[:sigLen]))
	}
	return signature[:sigLen]
}

func (s *ThreshsignEnclave) AggregateSigAndVerify(message []byte, ctrId uint32, newValue uint64, sigs map[peerpb.PeerID][]byte) []byte {
	signature := make([]byte, 256)
	sigLen := len(signature)

	hash := sha256.Sum256(message)

	cSigshares := make([]byte, 0, len(sigs))
	cSigshareLens := make([]C.unsigned, 0, len(sigs))
	cSigindex := make([]C.int, 0, len(sigs))
	for idx, sigshare := range sigs {
		cSigshares = append(cSigshares, sigshare...)
		cSigshareLens = append(cSigshareLens, C.unsigned(len(sigshare)))
		cSigindex = append(cSigindex, C.int(idx+1))
	}

	code := C.threshsign_aggregate_and_verify(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.unsigned(len(hash)),
		C.uint32_t(ctrId),
		C.uint64_t(newValue),
		C.unsigned(len(sigs)),
		(*C.char)(unsafe.Pointer(&cSigshares[0])),
		(*C.unsigned)(&cSigshareLens[0]),
		(*C.int)(&cSigindex[0]),
		(*C.char)(unsafe.Pointer(&signature[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLen)))

	if code != 0 {
		panic(errors.Errorf("unable to aggregatesigandverify. return code: %v", code))
	}

	return signature[:sigLen]
}

func (s *ThreshsignEnclave) Verify(message []byte, ctrId uint32, newValue uint64, sig []byte) bool {
	hash := sha256.Sum256(message)

	ret := C.threshsign_verify(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.unsigned(len(hash)),
		C.uint32_t(ctrId),
		C.uint64_t(newValue),
		(*C.char)(unsafe.Pointer(&sig[0])),
		C.unsigned(len(sig)))
	if ret != 0 {
		panic("Unable to verify signature")
	}
	return true
}
