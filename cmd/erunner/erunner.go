package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/ibalajiarun/go-consensus/erunner"
	"github.com/ibalajiarun/go-consensus/erunner/config"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func main() {
	logger := logger.NewDefaultLogger()

	var cmdGenerate = &cobra.Command{
		Use:   "generate [experiment config file] [experiment plan file]",
		Short: "Generate detailed experiment config from simple config",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			confFile := args[0]
			logger.Infof("Reading config: %s", confFile)
			confBytes, err := ioutil.ReadFile(confFile)
			if err != nil {
				logger.Fatalf("Error reading configuration file %s: %v", confFile, err)
			}

			expConfig := config.ExperimentConfig{}
			err = yaml.Unmarshal(confBytes, &expConfig)
			if err != nil {
				logger.Fatalf("Error unmarshaling configuration file: %v", err)
			}

			detailConfig := config.GenerateDetailConfig(expConfig)

			detailConfigBytes, err := yaml.Marshal(detailConfig)
			if err != nil {
				logger.Fatalf("Unable to marshal detailed config: %v", err)
			}

			logger.Infof("Writing plan file: %s", args[1])
			err = ioutil.WriteFile(args[1], detailConfigBytes, 0644)
			if err != nil {
				logger.Fatalf("Unable to write  detailed config to file: %v", err)
			}
		},
	}

	var algSkipCount, batchSkipCount int

	var cmdRun = &cobra.Command{
		Use:   "run [exp plan file]",
		Short: "Run the experiment with the plan file",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			planFile := args[0]

			logger.Infof("Reading config: %s", planFile)
			planBytes, err := ioutil.ReadFile(planFile)
			if err != nil {
				logger.Fatalf("Error reading configuration file %s: %v", planFile, err)
			}

			plan := config.ExperimentPlan{}
			err = yaml.Unmarshal(planBytes, &plan)
			if err != nil {
				logger.Fatalf("Error unmarshaling configuration file: %v", err)
			}

			logger.Infof("Creating data directory %s", plan.DataDirPrefix)
			if _, err := os.Lstat(plan.DataDirPrefix); err == nil {
				logger.Fatalf("Error creating directory. Directory already exists %s", plan.DataDirPrefix)
			}
			if err := os.MkdirAll(plan.DataDirPrefix, 0775); err != nil {
				logger.Fatalf("Error creating directory %v: %w", plan.DataDirPrefix, err)
			}

			logFile, err := os.Create(fmt.Sprintf("%s/out.log", plan.DataDirPrefix))
			if err != nil {
				logger.Fatalf("Error opening log output file: %v", err)
			}
			defer logFile.Close()
			logger.SetOutput(io.MultiWriter(logFile, os.Stderr))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			exp := erunner.New(plan, logger)

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGUSR1, syscall.SIGUSR2)
			go func() {
				for {
					sig := <-c
					switch sig {
					case syscall.SIGUSR1:
						logger.Info("Skipping All Algorithms in Batch...")
						exp.SkipAllAlgorithmForBatch()
					case syscall.SIGUSR2:
						logger.Info("Skipping Current Algorithm...")
						exp.SkipCurrentAlgorithm()
					case syscall.SIGINT:
						logger.Info("Received interrupt. Cancelling...")
						cancel()
					}
				}
			}()

			exp.Run(ctx, batchSkipCount, algSkipCount)
		},
	}

	cmdRun.Flags().IntVar(&batchSkipCount, "skip-batches", 0, "skip given number of algorithms")
	cmdRun.Flags().IntVar(&algSkipCount, "skip-algorithms", 0, "skip given number of algorithms")

	var rootCmd = &cobra.Command{Use: "app"}
	rootCmd.AddCommand(cmdGenerate, cmdRun)
	rootCmd.Execute()
}
