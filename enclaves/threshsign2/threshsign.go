package threshsign2

/*
#include "enclave_threshsign/untrusted/threshsign.h"
#cgo LDFLAGS: -L${SRCDIR}/enclave_threshsign -lthreshsign2
#cgo CFLAGS: -I/opt/sgxsdk/include
*/
import "C"

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unsafe"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/pkg/errors"
)

const enclaveFileName = "threshsign2enclave.signed.so"

type ThreshsignEnclave struct {
	filepath  string
	threshold uint32
	total     uint32
	cPtr      *C.threshsign_t
	pubKey    *pubKey
}

func NewThreshsignEnclave(filepath string, threshold, total uint32) *ThreshsignEnclave {
	return &ThreshsignEnclave{
		filepath:  filepath,
		threshold: threshold,
		total:     total,
	}
}

func (s *ThreshsignEnclave) Init(index int, seckey, pubkey string, fastLagrange bool) {
	fmt.Println("Initializing SGX Enclave") //, pkt_bytes, len(pkt_bytes))

	cFilepath := C.CString(fmt.Sprintf("%s/%s", s.filepath, enclaveFileName))
	cSecKey := C.CString(seckey)
	cPubKey := C.CString(pubkey)
	code := C.threshsign2_init(cFilepath,
		C.unsigned(s.threshold),
		C.unsigned(s.total),
		C.uint32_t(index),
		cSecKey,
		cPubKey,
		C.bool(fastLagrange),
		&s.cPtr)
	if code != 0 {
		panic(errors.Errorf("unable to initialize enclave. return code: %x", code))
	}

	C.free(unsafe.Pointer(cFilepath))
	C.free(unsafe.Pointer(cSecKey))
	C.free(unsafe.Pointer(cPubKey))

	fields := strings.Split(pubkey, ":")
	s.pubKey = newPubKey(fields)

	fmt.Println("Init done")
}

func (s *ThreshsignEnclave) Sign(message []byte, tc interface{}) []byte {
	signature := make([]byte, 256)
	sigLen := C.unsigned(len(signature))
	hash := sha256.Sum256(message)

	val, ok := tc.(uint64)
	if !ok {
		val = uint64(tc.(int))
	}

	ret := C.threshsign2_sign(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.uint64_t(val),
		(*C.char)(unsafe.Pointer(&signature[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLen)))

	if ret != 0 {
		panic("Unable to sign")
	}
	return signature[:sigLen]
}

func (s *ThreshsignEnclave) SignUsingEnclave(message []byte, tc interface{}) []byte {
	signature := make([]byte, 256)
	sigLen := C.unsigned(len(signature))
	hash := sha256.Sum256(message)

	val, ok := tc.(uint64)
	if !ok {
		val = uint64(tc.(int))
	}

	ret := C.threshsign2_sign_using_enclave(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.uint64_t(val),
		(*C.char)(unsafe.Pointer(&signature[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLen)))

	if ret != 0 {
		panic("Unable to sign using enclave")
	}
	return signature[:sigLen]
}

func (s *ThreshsignEnclave) AggregateSigAndVerify(message []byte, tc interface{}, sigs map[peerpb.PeerID][]byte) []byte {
	signature := make([]byte, 256)
	sigLen := C.unsigned(len(signature))

	hash := sha256.Sum256(message)

	val, ok := tc.(uint64)
	if !ok {
		val = uint64(tc.(int))
	}

	cSigshares := make([]byte, 0, len(sigs)*256)
	cSigshareLens := make([]C.unsigned, 0, len(sigs))
	cSigindex := make([]C.int, 0, len(sigs))
	for idx, sigshare := range sigs {
		cSigshares = append(cSigshares, sigshare...)
		cSigshareLens = append(cSigshareLens, C.unsigned(len(sigshare)))
		cSigindex = append(cSigindex, C.int(idx))
	}

	code := C.threshsign2_aggregate_and_verify(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.uint64_t(val),
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

func (s *ThreshsignEnclave) Verify(message []byte, tc interface{}, sig []byte) bool {
	hash := sha256.Sum256(message)

	val, ok := tc.(uint64)
	if !ok {
		val = uint64(tc.(int))
	}

	ret := C.threshsign2_verify(
		s.cPtr,
		(*C.char)(unsafe.Pointer(&hash[0])),
		C.uint64_t(val),
		(*C.char)(unsafe.Pointer(&sig[0])),
		C.unsigned(len(sig)))

	if ret != 0 {
		panic(fmt.Errorf("Unable to verify signature. return code: %v", ret))
	}
	return true
}
