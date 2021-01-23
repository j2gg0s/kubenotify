package client

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func buildOutofClusterConfig() (*rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

func newKubeInCluster() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can not get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can not create kubernetes client: %w", err)
	}

	return clientset, nil
}

func newKubeOutofCluster() (kubernetes.Interface, error) {
	config, err := buildOutofClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can not get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can not get kubernetes config: %w", err)
	}

	return clientset, nil
}

func NewKubeClient(outofCluster bool) (kubernetes.Interface, error) {
	if outofCluster {
		return newKubeOutofCluster()
	}

	if _, err := rest.InClusterConfig(); err != nil {
		return newKubeOutofCluster()
	}

	return newKubeInCluster()
}
