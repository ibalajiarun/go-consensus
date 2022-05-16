package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/types"
	"sigs.k8s.io/yaml"
)

const testConfig = `spec_file_path: "deploy/kube/erunner.yaml"
run_config_path: "config.yaml"
data_dir_prefix: "exp_data/n193_batching_0422_5"
kube_config_path: "deploy/ansible/k3s.yaml"
node_count: 193
algorithms:
  - name: Dispel
    max_failures: 64
    key_file: "keys/hybster.key.txt"
  - name: RCC
    max_failures: 64
    key_file: "keys/hybster.key.txt"
request_counts:
- client_counts: [19,38]
  inflight_request_counts: [1]
- client_counts: [19]
  inflight_request_counts: [5,10]
client_batch_counts: [10, 50]
payload_size: 500
datapoint_time: 2m
scale_datapoint_time: 1m`

var outConfig = ExperimentPlan{
	Count: 16,
	ExperimentCommonConfig: ExperimentCommonConfig{
		SpecFilePath:       "deploy/kube/erunner.yaml",
		RunConfigPath:      "config.yaml",
		DataDirPrefix:      "exp_data/n193_batching_0422_5",
		KubeConfigPath:     "deploy/ansible/k3s.yaml",
		NodeCount:          193,
		PayloadSize:        500,
		DatapointTime:      types.Duration{Duration: 2 * time.Minute},
		ScaleDatapointTime: types.Duration{Duration: 1 * time.Minute},
	},
	Batches: []PerBatchPlan{
		{
			BatchSize: 10,
			Algorithms: []PerAlgPlan{
				{
					Name:        peerpb.Algorithm_Dispel,
					MaxFailures: 64,
					KeyFile:     "keys/hybster.key.txt",
					InflightRequestCounts: []PerInflightPlan{
						{
							InflightRequestCount: 1,
							ClientCounts:         []int32{19, 38},
						},
						{
							InflightRequestCount: 5,
							ClientCounts:         []int32{19},
						},
						{
							InflightRequestCount: 10,
							ClientCounts:         []int32{19},
						},
					},
				},
				{
					Name:        peerpb.Algorithm_RCC,
					MaxFailures: 64,
					KeyFile:     "keys/hybster.key.txt",
					InflightRequestCounts: []PerInflightPlan{
						{
							InflightRequestCount: 1,
							ClientCounts:         []int32{19, 38},
						},
						{
							InflightRequestCount: 5,
							ClientCounts:         []int32{19},
						},
						{
							InflightRequestCount: 10,
							ClientCounts:         []int32{19},
						},
					},
				},
			},
		},
		{
			BatchSize: 50,
			Algorithms: []PerAlgPlan{
				{
					Name:        peerpb.Algorithm_Dispel,
					MaxFailures: 64,
					KeyFile:     "keys/hybster.key.txt",
					InflightRequestCounts: []PerInflightPlan{
						{
							InflightRequestCount: 1,
							ClientCounts:         []int32{19, 38},
						},
						{
							InflightRequestCount: 5,
							ClientCounts:         []int32{19},
						},
						{
							InflightRequestCount: 10,
							ClientCounts:         []int32{19},
						},
					},
				},
				{
					Name:        peerpb.Algorithm_RCC,
					MaxFailures: 64,
					KeyFile:     "keys/hybster.key.txt",
					InflightRequestCounts: []PerInflightPlan{
						{
							InflightRequestCount: 1,
							ClientCounts:         []int32{19, 38},
						},
						{
							InflightRequestCount: 5,
							ClientCounts:         []int32{19},
						},
						{
							InflightRequestCount: 10,
							ClientCounts:         []int32{19},
						},
					},
				},
			},
		},
	},
}

func TestGenerateDetailConfig(t *testing.T) {
	config := ExperimentConfig{}
	err := yaml.Unmarshal([]byte(testConfig), &config)
	if err != nil {
		t.Fatal(err)
	}
	detailConfig := GenerateDetailConfig(config)

	if !reflect.DeepEqual(outConfig, detailConfig) {
		t.Fatalf("not equal: \n %v !=\n %v", detailConfig, outConfig)
	}
	if _, err := yaml.Marshal(detailConfig); err != nil {
		t.Fatal(err)
	}
}
