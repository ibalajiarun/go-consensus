package threshsign

/*
#include "enclave_threshsign/untrusted/BatchThreshsign.h"
#cgo LDFLAGS: -L${SRCDIR}/enclave_threshsign -lthreshsign
#cgo CFLAGS: -I/opt/sgxsdk/include
*/
import "C"
import (
	"crypto/sha256"
	"unsafe"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

type ThreshsignRequest struct {
	Message     []byte
	Counter     uint64
	SigShares   map[peerpb.PeerID][]byte
	CombinedSig []byte
}

func (s *ThreshsignEnclave) BatchSign(reqBatch []ThreshsignRequest) [][]byte {
	hashes := make([]byte, len(reqBatch)*32)
	counterIds := make([]C.uint32_t, len(reqBatch))
	counters := make([]C.uint64_t, len(reqBatch))
	sigBatch := make([]byte, len(reqBatch)*256)
	sigLens := make([]C.unsigned, len(reqBatch))

	for i, r := range reqBatch {
		hash := sha256.Sum256(r.Message)
		copy(hashes[32*i:32*(i+1)], hash[:])
		counters[i] = C.uint64_t(r.Counter)
		sigLens[i] = 256
	}

	ret := C.threshsign_batch_sign(
		s.cPtr,
		C.unsigned(len(reqBatch)),
		(*C.char)(unsafe.Pointer(&hashes[0])),
		(*C.uint32_t)(&counterIds[0]),
		(*C.uint64_t)(&counters[0]),
		(*C.char)(unsafe.Pointer(&sigBatch[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLens[0])),
	)

	if ret != 0 {
		panic("Unable to sign")
	}

	retSigs := make([][]byte, len(reqBatch))
	for i := range retSigs {
		sigLen := int(sigLens[i])
		retSigs[i] = sigBatch[256*i : (256*i)+sigLen]
	}
	return retSigs
}

func (s *ThreshsignEnclave) BatchSignUsingEnclave(reqBatch []ThreshsignRequest) [][]byte {
	hashes := make([]byte, len(reqBatch)*32)
	counterIds := make([]C.uint32_t, len(reqBatch))
	counters := make([]C.uint64_t, len(reqBatch))
	sigBatch := make([]byte, len(reqBatch)*256)
	sigLens := make([]C.unsigned, len(reqBatch))

	for i, r := range reqBatch {
		hash := sha256.Sum256(r.Message)
		copy(hashes[32*i:32*(i+1)], hash[:])
		counterIds[i] = C.uint32_t(r.Counter)
		counters[i] = C.uint64_t(r.Counter)
		sigLens[i] = 256
	}

	ret := C.threshsign_batch_sign_using_enclave_batch(
		s.cPtr,
		C.unsigned(len(reqBatch)),
		(*C.char)(unsafe.Pointer(&hashes[0])),
		(*C.uint32_t)(&counterIds[0]),
		(*C.uint64_t)(&counters[0]),
		(*C.char)(unsafe.Pointer(&sigBatch[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLens[0])),
	)

	if ret != 0 {
		panic("Unable to sign using enclave")
	}

	retSigs := make([][]byte, len(reqBatch))
	for i := range retSigs {
		sigLen := int(sigLens[i])
		retSigs[i] = sigBatch[256*i : (256*i)+sigLen]
	}
	return retSigs
}

func (s *ThreshsignEnclave) BatchAggregateSigAndVerify(reqBatch []ThreshsignRequest) [][]byte {
	hashes := make([]byte, len(reqBatch)*32)
	counterIds := make([]C.uint32_t, len(reqBatch))
	counters := make([]C.uint64_t, len(reqBatch))
	sigBatch := make([]byte, len(reqBatch)*256)
	sigLens := make([]C.unsigned, len(reqBatch))

	cSigsharesBatch := make([]byte, 0, len(reqBatch))
	cSigshareBatchLens := make([]C.unsigned, 0, len(reqBatch))
	cSigshareBatchCounts := make([]C.unsigned, 0, len(reqBatch))
	cSigshareLens := make([]C.unsigned, 0, len(reqBatch))
	cSigindex := make([]C.int, 0, len(reqBatch))

	for i, r := range reqBatch {
		hash := sha256.Sum256(r.Message)
		copy(hashes[32*i:32*(i+1)], hash[:])
		counters[i] = C.uint64_t(r.Counter)
		sigLens[i] = 256

		totalLen := 0
		for idx, sigshare := range r.SigShares {
			cSigsharesBatch = append(cSigsharesBatch, sigshare...)
			cSigshareLens = append(cSigshareLens, C.unsigned(len(sigshare)))
			cSigindex = append(cSigindex, C.int(idx+1))
			totalLen += len(sigshare)
		}
		cSigshareBatchLens = append(cSigshareBatchLens, C.unsigned(totalLen))
		cSigshareBatchCounts = append(cSigshareBatchCounts, C.unsigned(len(r.SigShares)))
	}

	ret := C.threshsign_batch_aggregate_and_verify(
		s.cPtr,
		C.unsigned(len(reqBatch)),
		(*C.char)(unsafe.Pointer(&hashes[0])),
		(*C.uint32_t)(&counterIds[0]),
		(*C.uint64_t)(&counters[0]),
		(*C.char)(unsafe.Pointer(&cSigsharesBatch[0])),
		(*C.unsigned)(&cSigshareBatchLens[0]),
		(*C.unsigned)(&cSigshareBatchCounts[0]),
		(*C.unsigned)(&cSigshareLens[0]),
		(*C.int)(&cSigindex[0]),
		(*C.char)(unsafe.Pointer(&sigBatch[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLens[0])),
	)

	if ret != 0 {
		panic("Unable to aggregate and verify")
	}

	retSigs := make([][]byte, len(reqBatch))
	for i := range retSigs {
		sigLen := int(sigLens[i])
		retSigs[i] = sigBatch[256*i : (256*i)+sigLen]
		// fmt.Printf("agg_sig %v %s\n", sigLen, retSigs[i])
	}
	return retSigs
}

func (s *ThreshsignEnclave) BatchVerify(reqBatch []ThreshsignRequest) bool {
	hashes := make([]byte, len(reqBatch)*32)
	counterIds := make([]C.uint32_t, len(reqBatch))
	counters := make([]C.uint64_t, len(reqBatch))
	sigBatch := make([]byte, 0, len(reqBatch))
	sigLens := make([]C.unsigned, len(reqBatch))

	for i, r := range reqBatch {
		hash := sha256.Sum256(r.Message)
		copy(hashes[32*i:32*(i+1)], hash[:])
		counters[i] = C.uint64_t(r.Counter)

		// fmt.Printf("verify_agg_sig %p %v %s\n", s.cPtr, len(r.CombinedSig), r.CombinedSig)
		// os.Stdout.Sync()
		sigBatch = append(sigBatch, r.CombinedSig...)
		sigLens[i] = C.unsigned(len(r.CombinedSig))
	}

	ret := C.threshsign_batch_verify(
		s.cPtr,
		C.unsigned(len(reqBatch)),
		(*C.char)(unsafe.Pointer(&hashes[0])),
		(*C.uint32_t)(&counterIds[0]),
		(*C.uint64_t)(&counters[0]),
		(*C.char)(unsafe.Pointer(&sigBatch[0])),
		(*C.unsigned)(unsafe.Pointer(&sigLens[0])),
	)

	if ret != 0 {
		panic("Unable to verify")
	}

	return true
}
