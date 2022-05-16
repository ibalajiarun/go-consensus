package erunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"time"

	"github.com/ibalajiarun/go-consensus/erunner/config"
	"github.com/ibalajiarun/go-consensus/erunner/discovery"
	"github.com/ibalajiarun/go-consensus/erunner/kube"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"
)

type Runner struct {
	plan   config.ExperimentPlan
	logger logger.Logger
	specs  []kube.SpecKind

	skipBatchC     chan struct{}
	skipAlgorithmC chan struct{}
}

func parseSpecFile(stream []byte) ([]kube.SpecKind, error) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode

	rawSpecs := bytes.Split(stream, []byte("---"))

	specs := make([]kube.SpecKind, len(rawSpecs))
	for i, rawSpec := range rawSpecs {
		obj, groupKindVersion, err := decode(rawSpec, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("unable to decode spec %d: %w. %s", i, err, rawSpec)
		}
		specs[i] = kube.SpecKind{Spec: obj, Kind: groupKindVersion.Kind}
	}
	return specs, nil
}

func New(plan config.ExperimentPlan, logger logger.Logger) *Runner {
	stream, err := ioutil.ReadFile(plan.SpecFilePath)
	if err != nil {
		logger.Fatalf("Unable to read specification file %v: %w", plan.SpecFilePath, err)
	}
	specs, err := parseSpecFile(stream)
	if err != nil {
		logger.Fatalf("Error parsing specification: %w", err)
	}
	logger.Infof("Parsed specification file. Found %d configs.", len(specs))
	for i := range specs {
		logger.Infof("Specification %d, Kind: %s", i+1, specs[i].Kind)
	}

	return &Runner{
		plan:   plan,
		logger: logger,
		specs:  specs,

		skipBatchC:     make(chan struct{}),
		skipAlgorithmC: make(chan struct{}),
	}
}

func (r *Runner) Run(ctx context.Context, batchSkipCount, algSkipCount int) {
	r.logger.Infof("Experiment started on %s", time.Now().Local().String())

	rawConfig, err := ioutil.ReadFile(r.plan.RunConfigPath)
	if err != nil {
		r.logger.Fatalf("Unable to read run config file %v: %w", r.plan.SpecFilePath, err)
	}
	runConfig := discovery.Config{}
	err = yaml.Unmarshal(rawConfig, &runConfig)
	if err != nil {
		r.logger.Fatalf("Error unmarshaling configuration file: %v", err)
	}

	r.logger.Info("Preparing for the experiment...")

	r.logger.Info("Writing raw run config")
	err = ioutil.WriteFile(fmt.Sprintf("%s/run_exp_config.yaml", r.plan.DataDirPrefix), rawConfig, 0644)
	if err != nil {
		r.logger.Fatalf("Unable to write run config file to %v: %w", r.plan.DataDirPrefix, err)
	}

	data, err := yaml.Marshal(r.plan)
	if err != nil {
		r.logger.Fatalf("Unable to marshall experiment config: %v: %w", r.plan.DataDirPrefix, err)
	}
	if err := ioutil.WriteFile(fmt.Sprintf("%s/run_config.yaml", r.plan.DataDirPrefix), data, 0644); err != nil {
		r.logger.Fatalf("Unable to write experiment config: %v: %w", r.plan.DataDirPrefix, err)
	}

	r.logger.Infof("Running Data Series Experiments (Total : %d)...", r.plan.Count)
	runConfig.PeerCount = r.plan.NodeCount
	runConfig.ClientConfig.RequestPayloadSize = uint64(r.plan.PayloadSize)

	lines := make([][]*DataPoint, 0, r.plan.Count)
	var algNames []string

	for bi, batch := range r.plan.Batches {
		if bi < batchSkipCount {
			continue
		}
		batchSkipCount = 0

		runConfig.ClientConfig.RequestOpsBatchSize = uint64(batch.BatchSize)
		for i, alg := range batch.Algorithms {
			if i < algSkipCount {
				continue
			}
			algSkipCount = 0

			seriesName := fmt.Sprintf("%d-%s-b%d", i, alg.Name.String(), batch.BatchSize)

			runConfig.Algorithm = alg.Name
			runConfig.MaxFailures = int32(alg.MaxFailures)
			runConfig.KeyFile = alg.KeyFile
			runConfig.ClientConfig.FastResponsePercent = uint64(alg.FastResponsePercent)
			runConfig.ServerConfig.MultiChainDuoBFTConfig.InstancesPerPeer = uint32(alg.NumChains)

			expDirPrefix := fmt.Sprintf("%s/%s", r.plan.DataDirPrefix, seriesName)
			s, err := r.runSeriesExp(ctx, expDirPrefix, alg, runConfig)
			if err != nil {
				r.logger.Errorf("Experiment Failed: %w", err)
				goto exit
			}

			lines = append(lines, s)
			algNames = append(algNames, seriesName)

			select {
			case <-r.skipBatchC:
				r.logger.Infof("Skipping all Algorithm for batch size: %d", batch.BatchSize)
				break
			default:
			}
		}
	}
exit:
	// Write all lines to file
	file, err := os.Create(fmt.Sprintf("%s/plot-data.csv", r.plan.DataDirPrefix))
	if err != nil {
		r.logger.Fatalf("Error creating file to write plot data: %w", err)
	}
	defer file.Close()
	writeLines(file, algNames, lines)

	// Generate plot
	plt, err := plotXY(algNames, lines)
	if err != nil {
		r.logger.Fatalf("Plot error: %w", err)
	}
	// Save the plot to a PNG file.
	if err := plt.Save(4*vg.Inch, 4*vg.Inch, fmt.Sprintf("%s/plot.png", r.plan.DataDirPrefix)); err != nil {
		r.logger.Fatalf("Unable to save plot: %w", err)
	}

	r.logger.Info("Finished Running Experiments")
}

func (r *Runner) SkipCurrentAlgorithm() {
	r.skipAlgorithmC <- struct{}{}
}

func (r *Runner) SkipAllAlgorithmForBatch() {
	r.skipBatchC <- struct{}{}
}

func writeLines(w io.Writer, algos []string, lines [][]*DataPoint) {
	for i, line := range lines {
		fmt.Fprintf(w, "lat,%s", algos[i])
		for _, pt := range line {
			fmt.Fprintf(w, ",%f", pt.Latency)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "tps,%s", algos[i])
		for _, pt := range line {
			fmt.Fprintf(w, ",%f", pt.Throughput)
		}
		fmt.Fprintln(w)
	}
}

func plotXY(algos []string, lines [][]*DataPoint) (*plot.Plot, error) {
	p, err := plot.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create plot: %w", err)
	}

	p.Title.Text = "Throughput vs Latency"
	p.X.Label.Text = "Throughput"
	p.Y.Label.Text = "Latency"
	p.Y.Min = 0

	vs := make([]interface{}, 0, 2*len(lines))
	for i, line := range lines {
		pts := make(plotter.XYs, len(line))
		for i, pt := range line {
			pts[i].X = pt.Throughput
			lat := pt.Latency
			if math.IsNaN(lat) {
				lat = 0
			}
			pts[i].Y = lat
		}

		vs = append(vs, algos[i], pts)
	}

	err = plotutil.AddLinePoints(p, vs...)
	if err != nil {
		return nil, fmt.Errorf("unable to create add line to plot: %w", err)
	}
	p.Legend.Top = true
	p.Legend.Left = false

	return p, nil
}
