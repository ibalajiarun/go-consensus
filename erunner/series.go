package erunner

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ibalajiarun/go-consensus/erunner/config"
	"github.com/ibalajiarun/go-consensus/erunner/discovery"
)

func (r *Runner) runSeriesExp(ctx context.Context, dirPrefix string, alg config.PerAlgPlan, runConfig discovery.Config) (dp []*DataPoint, err error) {
	r.logger.Infof("Series Experiment %s", alg.Name.String())

	r.logger.Infof("Creating Directory %s", dirPrefix)
	if _, err := os.Lstat(dirPrefix); err == nil {
		return nil, fmt.Errorf("directory already exists %v: %w", dirPrefix, err)
	}
	if err := os.Mkdir(dirPrefix, 0775); err != nil {
		return nil, fmt.Errorf("error creating directory %v: %w", dirPrefix, err)
	}

	var points []*DataPoint

	defer func() {
		if err != nil {
			return
		}

		file, ferr := os.Create(fmt.Sprintf("%s/series.csv", dirPrefix))
		if ferr != nil {
			dp = nil
			err = fmt.Errorf("error creating file to write data: %w", err)
			return
		}
		defer file.Close()

		for _, pt := range points {
			fmt.Fprintf(file, "%f,%f\n", pt.Latency, pt.Throughput)
		}

		r.logger.Info("Finished Running Data Series Experiments")
	}()

	for _, inf := range alg.InflightRequestCounts {
		runConfig.ClientConfig.MaxInflightRequests = uint32(inf.InflightRequestCount)
		runConfig.ClientCount = inf.ClientCounts[0]

		startFrom := 0
	for_loop_1:
		for {
			dps, err := r.runDatapointExp(ctx, dirPrefix, inf, runConfig, startFrom)

			if dps != nil {
				points = append(points, dps...)
			}

			if err != nil {
				if errors.Is(err, ErrThroughputZero) {
					break
				}
				r.logger.Errorf("error running datapoint experiment: %w", err)
				return points, fmt.Errorf("error running datapoint experiment: %w", err)
			}

			startFrom += len(dps)
			if startFrom == len(inf.ClientCounts) {
				break
			}

			select {
			case <-r.skipBatchC:
				r.logger.Infof("Skipping current Algorithm: %s", dirPrefix)
				break for_loop_1
			case <-ctx.Done():
				r.logger.Infof("cancelling experiment: %w", ctx.Err())
				return points, ctx.Err()
			default:
			}
		}
	}

	return points, nil
}
