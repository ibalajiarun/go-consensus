package discovery

import (
	"testing"

	"sigs.k8s.io/yaml"
)

var data = `
leader_region: "Virginia"
server:
  algorithm: 4
  max_failures: 1
  max_fast_failures: 1
  key_file: ""
  enclave_path: ""
client:
  client_count: 2
  threads_per_client: 2
  experiment: latency # or throughput
  client_type: "normal" # normal, zyzzyva, sbft, sdbft, dester
  req_key_size: 2
  req_value_size: 2
`

func TestConfig(t *testing.T) {
	c := Config{}

	err := yaml.Unmarshal([]byte(data), &c)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	t.Logf("--- c:\n%v\n\n", c)

	d, err := yaml.Marshal(&c)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	t.Logf("--- c dump:\n%s\n\n", string(d))

	t.Fail()
}
