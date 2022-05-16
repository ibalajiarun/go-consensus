package config

import (
	"github.com/ibalajiarun/go-consensus/peer/peerpb"
	"github.com/ibalajiarun/go-consensus/pkg/types"
)

type ExperimentCommonConfig struct {
	SpecFilePath       string         `json:"spec_file_path"`
	RunConfigPath      string         `json:"run_config_path"`
	DataDirPrefix      string         `json:"data_dir_prefix"`
	KubeConfigPath     string         `json:"kube_config_path"`
	NodeCount          int32          `json:"node_count"`
	PayloadSize        int32          `json:"payload_size"`
	DatapointTime      types.Duration `json:"datapoint_time"`
	ScaleDatapointTime types.Duration `json:"scale_datapoint_time"`
}

type ExperimentConfig struct {
	ExperimentCommonConfig
	Algorithms []struct {
		Name                peerpb.Algorithm `json:"name"`
		MaxFailures         uint64           `json:"max_failures"`
		KeyFile             string           `json:"key_file"`
		FastResponsePercent int              `json:"fast_response_percent"`
		NumChains           int              `json:"num_chains"`
	} `json:"algorithms"`
	RequestCounts []struct {
		InflightRequestCounts []int32 `json:"inflight_request_counts"`
		ClientCounts          []int32 `json:"client_counts"`
	} `json:"request_counts"`
	ClientBatchCounts []int32 `json:"client_batch_counts"`
}

type PerInflightPlan struct {
	InflightRequestCount int32   `json:"inflight_request_count"`
	ClientCounts         []int32 `json:"client_counts"`
}

type PerAlgPlan struct {
	Name                  peerpb.Algorithm  `json:"name"`
	MaxFailures           uint64            `json:"max_failures"`
	KeyFile               string            `json:"key_file"`
	InflightRequestCounts []PerInflightPlan `json:"inflight_request_counts"`
	FastResponsePercent   int               `json:"fast_response_percent"`
	NumChains             int               `json:"num_chains"`
}

type PerBatchPlan struct {
	BatchSize  int32        `json:"batch_size"`
	Algorithms []PerAlgPlan `json:"algorithms"`
}

type ExperimentPlan struct {
	ExperimentCommonConfig
	Count   int            `json:"count"`
	Batches []PerBatchPlan `json:"batches"`
}

func GenerateDetailConfig(config ExperimentConfig) ExperimentPlan {
	dConfig := ExperimentPlan{
		ExperimentCommonConfig: config.ExperimentCommonConfig,
	}

	count := 0
	for _, b := range config.ClientBatchCounts {
		batchConfig := PerBatchPlan{
			BatchSize: b,
		}
		for _, alg := range config.Algorithms {
			algConfig := PerAlgPlan{
				Name:                alg.Name,
				MaxFailures:         alg.MaxFailures,
				KeyFile:             alg.KeyFile,
				FastResponsePercent: alg.FastResponsePercent,
				NumChains:           alg.NumChains,
			}
			for _, reqPair := range config.RequestCounts {
				for _, inflightReqs := range reqPair.InflightRequestCounts {
					inflightConfig := PerInflightPlan{
						InflightRequestCount: inflightReqs,
						ClientCounts:         reqPair.ClientCounts,
					}
					count += len(reqPair.ClientCounts)
					algConfig.InflightRequestCounts = append(algConfig.InflightRequestCounts, inflightConfig)
				}
			}
			batchConfig.Algorithms = append(batchConfig.Algorithms, algConfig)
		}
		dConfig.Batches = append(dConfig.Batches, batchConfig)
	}
	dConfig.Count = count
	return dConfig
}
