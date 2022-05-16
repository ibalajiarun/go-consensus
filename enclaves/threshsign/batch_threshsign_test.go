package threshsign

import (
	"fmt"
	"os"
	"testing"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

func TestEnclaveThreshsignBatchOnlyThreshold(t *testing.T) {
	batch := 10
	threshold := uint32(7)
	total := uint32(13)

	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", threshold, total))

	enclavePrefix, ok := os.LookupEnv("workspaceFolder")
	if !ok {
		t.Fatalf("Please set the $workspaceFolder environment variable to point to the project root")
	}
	enclavePath := fmt.Sprintf("%v/enclaves/threshsign/enclave_threshsign/", enclavePrefix)

	var enclaves []*ThreshsignEnclave
	for index := 0; index < int(total); index++ {
		s := NewThreshsignEnclave(enclavePath, threshold, total)
		s.Init(index+1, skeys[index], vkeys, total)
		enclaves = append(enclaves, s)
	}

	sigsMap := make([]map[peerpb.PeerID][]byte, batch)
	reqs := make([]ThreshsignRequest, batch)
	for i := range reqs {
		sigsMap[i] = make(map[peerpb.PeerID][]byte)
		reqs[i] = ThreshsignRequest{
			Message: []byte(fmt.Sprintf("hello%v", i)),
			Counter: 100,
		}
	}
	c := uint32(0)
	for i, e := range enclaves {
		sigs := e.BatchSignUsingEnclave(reqs)
		for idx, sig := range sigs {
			sigsMap[idx][peerpb.PeerID(i)] = sig
		}
		c++
		if c >= threshold {
			break
		}
	}

	for i := range reqs {
		reqs[i].SigShares = sigsMap[i]
	}
	for _, e := range enclaves {
		aggSig := e.BatchAggregateSigAndVerify(reqs)
		for i, sig := range aggSig {
			reqs[i].CombinedSig = sig
		}
		e.BatchVerify(reqs)
	}
}

func TestThreshsignBatchOnlyThreshold(t *testing.T) {
	batch := 10
	threshold := uint32(7)
	total := uint32(13)

	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", threshold, total))

	enclavePrefix, ok := os.LookupEnv("workspaceFolder")
	if !ok {
		t.Fatalf("Please set the $workspaceFolder environment variable to point to the project root")
	}
	enclavePath := fmt.Sprintf("%v/enclaves/threshsign/enclave_threshsign/", enclavePrefix)

	var enclaves []*ThreshsignEnclave
	for index := 0; index < int(total); index++ {
		s := NewThreshsignEnclave(enclavePath, threshold, total)
		s.Init(index+1, skeys[index], vkeys, total)
		enclaves = append(enclaves, s)
	}

	sigsMap := make([]map[peerpb.PeerID][]byte, batch)
	reqs := make([]ThreshsignRequest, batch)
	for i := range reqs {
		sigsMap[i] = make(map[peerpb.PeerID][]byte)
		reqs[i] = ThreshsignRequest{
			Message: []byte(fmt.Sprintf("hello%v", i)),
			Counter: 100,
		}
	}
	c := uint32(0)
	for i, e := range enclaves {
		sigs := e.BatchSign(reqs)
		for idx, sig := range sigs {
			sigsMap[idx][peerpb.PeerID(i)] = sig
		}
		c++
		if c >= threshold {
			break
		}
	}

	for i := range reqs {
		reqs[i].SigShares = sigsMap[i]
	}
	for _, e := range enclaves {
		aggSig := e.BatchAggregateSigAndVerify(reqs)
		for i, sig := range aggSig {
			reqs[i].CombinedSig = sig
		}
		e.BatchVerify(reqs)
	}
}

func BenchmarkEnclaveThreshsignSignBatch(b *testing.B) {
	batch := 1
	threshold := uint32(7)
	total := uint32(13)

	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", threshold, total))

	enclavePrefix, ok := os.LookupEnv("workspaceFolder")
	if !ok {
		b.Fatalf("Please set the $workspaceFolder environment variable to point to the project root")
	}
	enclavePath := fmt.Sprintf("%v/enclaves/threshsign/enclave_threshsign/", enclavePrefix)

	e := NewThreshsignEnclave(enclavePath, threshold, total)
	e.Init(1, skeys[0], vkeys, total)

	reqs := make([]ThreshsignRequest, batch)
	for i := range reqs {
		reqs[i] = ThreshsignRequest{
			Message: []byte(fmt.Sprintf("hello%v", i)),
			Counter: 100,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.BatchSignUsingEnclave(reqs)
	}
}

func BenchmarkEnclaveThreshsignAggregateAndVerify(b *testing.B) {
	batch := 1
	threshold := uint32(7)
	total := uint32(13)

	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", threshold, total))

	enclavePrefix, ok := os.LookupEnv("workspaceFolder")
	if !ok {
		b.Fatalf("Please set the $workspaceFolder environment variable to point to the project root")
	}
	enclavePath := fmt.Sprintf("%v/enclaves/threshsign/enclave_threshsign/", enclavePrefix)

	var enclaves []*ThreshsignEnclave
	for index := 0; index < int(total); index++ {
		s := NewThreshsignEnclave(enclavePath, threshold, total)
		s.Init(index+1, skeys[index], vkeys, total)
		enclaves = append(enclaves, s)
	}

	sigsMap := make([]map[peerpb.PeerID][]byte, batch)
	reqs := make([]ThreshsignRequest, batch)
	for i := range reqs {
		sigsMap[i] = make(map[peerpb.PeerID][]byte)
		reqs[i] = ThreshsignRequest{
			Message: []byte(fmt.Sprintf("hello%v", i)),
			Counter: 100,
		}
	}
	c := uint32(0)
	for i, e := range enclaves {
		sigs := e.BatchSignUsingEnclave(reqs)
		for idx, sig := range sigs {
			sigsMap[idx][peerpb.PeerID(i)] = sig
		}
		c++
		if c >= threshold {
			break
		}
	}

	for i := range reqs {
		reqs[i].SigShares = sigsMap[i]
	}

	e := enclaves[0]
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = e.BatchAggregateSigAndVerify(reqs)
	}
}

func BenchmarkEnclaveThreshsignBatchVerify(b *testing.B) {
	batch := 1
	threshold := uint32(7)
	total := uint32(13)

	skeys, vkeys := readKeys(fmt.Sprintf("%d_%d", threshold, total))

	enclavePrefix, ok := os.LookupEnv("workspaceFolder")
	if !ok {
		b.Fatalf("Please set the $workspaceFolder environment variable to point to the project root")
	}
	enclavePath := fmt.Sprintf("%v/enclaves/threshsign/enclave_threshsign/", enclavePrefix)

	var enclaves []*ThreshsignEnclave
	for index := 0; index < int(total); index++ {
		s := NewThreshsignEnclave(enclavePath, threshold, total)
		s.Init(index+1, skeys[index], vkeys, total)
		enclaves = append(enclaves, s)
	}

	sigsMap := make([]map[peerpb.PeerID][]byte, batch)
	reqs := make([]ThreshsignRequest, batch)
	for i := range reqs {
		sigsMap[i] = make(map[peerpb.PeerID][]byte)
		reqs[i] = ThreshsignRequest{
			Message: []byte(fmt.Sprintf("hello%v", i)),
			Counter: 100,
		}
	}
	c := uint32(0)
	for i, e := range enclaves {
		sigs := e.BatchSignUsingEnclave(reqs)
		for idx, sig := range sigs {
			sigsMap[idx][peerpb.PeerID(i)] = sig
		}
		c++
		if c >= threshold {
			break
		}
	}

	for i := range reqs {
		reqs[i].SigShares = sigsMap[i]
	}
	for _, e := range enclaves {
		aggSig := e.BatchAggregateSigAndVerify(reqs)
		for i, sig := range aggSig {
			reqs[i].CombinedSig = sig
		}
	}

	e := enclaves[0]
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		e.BatchVerify(reqs)
	}
}
