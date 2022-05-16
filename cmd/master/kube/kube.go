package kube

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ibalajiarun/go-consensus/pkg/logger"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	tappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type KubeController struct {
	logger            logger.Logger
	config            *rest.Config
	serverDepManifest *appsv1.Deployment
	clientDepManifest *appsv1.Deployment
	dClient           tappsv1.DeploymentInterface
	clientset         *kubernetes.Clientset
}

func NewKubeDeploymentController(logger logger.Logger, nodeCount, clientCount int32, serverManifest, clientManifest, kubeConfig string) (*KubeController, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		logger.Fatalf("Unable to read kubeconfig file %s: %v", kubeConfig, err)
	}

	sDep, err := readManifest(serverManifest)
	if err != nil {
		logger.Fatalf("Unable to read manifest file %s: %v", serverManifest, err)
	}
	*sDep.Spec.Replicas = nodeCount

	cDep, err := readManifest(clientManifest)
	if err != nil {
		logger.Fatalf("Unable to read manifest file %s: %v", clientManifest, err)
	}
	*cDep.Spec.Replicas = clientCount

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dCli := clientset.AppsV1().Deployments(v1.NamespaceDefault)

	return &KubeController{
		logger:            logger,
		config:            config,
		serverDepManifest: sDep,
		clientDepManifest: cDep,
		dClient:           dCli,
		clientset:         clientset,
	}, nil
}

func (kc *KubeController) Apply() error {
	sDepResult, err := kc.dClient.Create(kc.serverDepManifest)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	kc.logger.Infof("Created server deployment %q.\n", sDepResult.GetObjectMeta().GetName())

	cDepResult, err := kc.dClient.Create(kc.clientDepManifest)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	kc.logger.Infof("Created client deployment %q.\n", cDepResult.GetObjectMeta().GetName())

	return nil
}

func readManifest(file string) (*appsv1.Deployment, error) {
	serverManifestBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	dep := &appsv1.Deployment{}

	err = yaml.Unmarshal(serverManifestBytes, dep)
	if err != nil {
		return nil, err
	}

	return dep, nil
}

func (kc *KubeController) Delete() {
	kc.logger.Info("Deleting deployment...")
	deletePolicy := v1.DeletePropagationForeground
	if err := kc.dClient.Delete(kc.serverDepManifest.Name, &v1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}
	if err := kc.dClient.Delete(kc.clientDepManifest.Name, &v1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}

	err := wait.PollImmediate(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		p, err := kc.clientset.CoreV1().Pods(v1.NamespaceDefault).List(v1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", kc.serverDepManifest.Spec.Template.Labels["app"])})
		if len(p.Items) == 0 || errors.IsNotFound(err) {
			kc.logger.Info("Pods removed")
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		kc.logger.Error(err)
	} else {
		kc.logger.Info("Deleted deployment.")
	}
}
