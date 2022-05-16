package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/ibalajiarun/go-consensus/cmd/master/discovery"
	"github.com/ibalajiarun/go-consensus/cmd/master/kube"
	"github.com/ibalajiarun/go-consensus/pkg/logger"
	flag "github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

var portnum *int = flag.IntP("port", "p", 7000, "Port # to listen on. Defaults to 7070")
var confFile *string = flag.StringP("config", "c", "config.yaml", "Path to config")
var kubeConfigFile *string = flag.StringP("kube-config", "k", "/etc/rancher/k3s/k3s.yaml", "Path to kubeconfig")
var prefix *string = flag.String("template-prefix", "app/master/", "default template prefix")
var noDeploy *bool = flag.Bool("no-deploy", false, "dont deploy, run master only.")

func main() {
	logger := logger.NewDefaultLogger()

	flag.Parse()

	logger.Infof("Reading config: %s", *confFile)
	confBytes, err := ioutil.ReadFile(*confFile)
	if err != nil {
		logger.Fatalf("Error reading configuration file %s: %v", *confFile, err)
	}

	config := &discovery.Config{}
	err = yaml.Unmarshal(confBytes, config)
	if err != nil {
		logger.Fatalf("Error unmarshaling configuration file: %v", err)
	}

	logger.Info(string(confBytes))
	logger.Info(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Infof("Starting discovery server on %d", *portnum)
	d, err := discovery.NewDiscoveryServer(*portnum, logger, config)
	if err != nil {
		logger.Error(err)
		return
	}
	go d.Serve(ctx)

	if !*noDeploy {
		k, _ := kube.NewKubeDeploymentController(logger,
			config.NodeCount,
			config.ClientCount,
			"deploy/local/kube/server-deployment.yaml",
			"deploy/local/kube/client-deployment.yaml",
			*kubeConfigFile)

		err = k.Apply()
		if err != nil {
			logger.Error(err)
			return
		}

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			d.Stop()
			k.Delete()
			os.Exit(0)
		}()
	}

	select {}
}
