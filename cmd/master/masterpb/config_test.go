package masterpb

import (
	"testing"

	"sigs.k8s.io/yaml"
)

func TestBaseClientConfig_UnmarshalJSON(t *testing.T) {
	raw := `max_inflight_requests: 1
request_ops_batch_size: 500
request_payload_size: 1024`

	var a BaseClientConfig
	b := BaseClientConfig{
		MaxInflightRequests: 1,
		RequestOpsBatchSize: 500,
		RequestPayloadSize:  1024,
	}
	err := yaml.Unmarshal([]byte(raw), &a)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("a != b: %v != %v", a, b)
	}
}
