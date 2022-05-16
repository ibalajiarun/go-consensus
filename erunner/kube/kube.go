package kube

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeManager struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
}

type SpecKind struct {
	Spec runtime.Object
	Kind string
}

func NewManager(kubeConfigPath string) (*KubeManager, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to read kubeconfig file at %s: %w", kubeConfigPath, err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Unable to create clientset: %w", err)
	}

	if _, err = clientset.CoreV1().Nodes().List(metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("unable to access cluster: %w", err)
	}

	return &KubeManager{
		config:    config,
		clientset: clientset,
	}, nil
}

func (km *KubeManager) GetAllNodeDetails() (*corev1.NodeList, error) {
	nodeClient := km.clientset.CoreV1().Nodes()
	return nodeClient.List(metav1.ListOptions{})
}

func (km *KubeManager) CreateService(namespace string, svc *corev1.Service) (*corev1.Service, error) {
	svcClient := km.clientset.CoreV1().Services(namespace)
	return svcClient.Create(svc)
}

func (km *KubeManager) DeleteService(namespace, svcName string) error {
	svcClient := km.clientset.CoreV1().Services(namespace)
	return svcClient.Delete(svcName, &metav1.DeleteOptions{})
}

func (km *KubeManager) CreateDeployment(namespace string, dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	depClient := km.clientset.AppsV1().Deployments(namespace)
	return depClient.Create(dep)
}

func (km *KubeManager) DeleteDeployment(namespace, depName string) error {
	depClient := km.clientset.AppsV1().Deployments(namespace)
	return depClient.Delete(depName, &metav1.DeleteOptions{})
}

func (km *KubeManager) CreateReplicaSet(namespace string, rs *appsv1.ReplicaSet) (*appsv1.ReplicaSet, error) {
	rsClient := km.clientset.AppsV1().ReplicaSets(namespace)
	return rsClient.Create(rs)
}

func (km *KubeManager) ScaleReplicaSet(
	namespace, rsName string,
	replicas int32,
) error {
	rsClient := km.clientset.AppsV1().ReplicaSets(namespace)
	scale := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind: "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsName,
			Namespace: namespace,
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: replicas,
		},
	}
	_, err := rsClient.UpdateScale(rsName, scale)
	return err
}

func (km *KubeManager) DeleteReplicaSet(namespace, rsName string) error {
	rsClient := km.clientset.AppsV1().ReplicaSets(namespace)
	deletePolicy := metav1.DeletePropagationForeground
	err := rsClient.Delete(rsName, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		return fmt.Errorf("error deleting replicaset: %w", err)
	}
	err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
		p, err := km.clientset.CoreV1().Pods(corev1.NamespaceDefault).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", rsName)})
		if len(p.Items) == 0 || apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for replicaset pods deleteion: %w", err)
	}
	err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
		p, err := km.clientset.AppsV1().ReplicaSets(corev1.NamespaceDefault).List(metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", rsName)})
		if len(p.Items) == 0 || apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for replicaset deleteion: %w", err)
	}
	return nil
}

func (km *KubeManager) MonitorReplicaSet(ctx context.Context,
	namespace, label string) error {
	opts := metav1.ListOptions{
		LabelSelector: label,
	}
	watcher, err := km.clientset.CoreV1().Pods(namespace).Watch(opts)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case event := <-watcher.ResultChan():
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Fatal("unexpected type")
			}

			previous := true
			switch event.Type {
			case watch.Modified:

			case watch.Error:
				req := km.clientset.CoreV1().Pods(namespace).
					GetLogs(pod.Name, &corev1.PodLogOptions{
						Previous: previous,
					})
				res, err := req.Do().Raw()
				if err != nil {
					return fmt.Errorf("could not get logs from failed pod %v: %w", pod.Name, err)
				}
				return fmt.Errorf("log from failed pod %v:\n%s", pod.Name, res)
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return ctx.Err()
		}
	}
}

func (km *KubeManager) ForceDeletePods(namespace, label string) error {
	podClient := km.clientset.CoreV1().Pods(namespace)
	deletePolicy := metav1.DeletePropagationForeground
	grace := int64(0)
	err := podClient.DeleteCollection(
		&metav1.DeleteOptions{
			GracePeriodSeconds: &grace,
			PropagationPolicy:  &deletePolicy,
		}, metav1.ListOptions{
			LabelSelector: label,
		},
	)
	if err != nil {
		return fmt.Errorf("error force deleting pods: %w", err)
	}
	return nil
}
