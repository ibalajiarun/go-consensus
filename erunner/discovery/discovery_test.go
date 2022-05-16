package discovery

import (
	"testing"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
)

func TestClientIDGeneration(t *testing.T) {
	var idGen int32
	regionPeers := []peerpb.PeerInfo{
		{
			PeerID: 2,
		},
		{
			PeerID: 8,
		},
		{
			PeerID: 10,
		},
		{
			PeerID: 12,
		},
		{
			PeerID: 14,
		},
		{
			PeerID: 18,
		},
	}
	peerCount := int32(19)
	expectedIDs := []int32{2, 8, 10, 12, 14, 18, 21, 27, 29, 31, 33, 37}

	for i, expID := range expectedIDs {
		idx := nextClientID(&idGen, regionPeers, peerCount)
		if idx != expID {
			t.Fatalf("unexpected id for index %v: expected: %v, actual: %v", i, expID, idx)
		}
	}
}
