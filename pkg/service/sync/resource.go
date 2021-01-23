package sync

import (
	"fmt"
	"strings"

	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	rbac_v1beta1 "k8s.io/api/rbac/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceMeta struct {
	Name       string
	Namespaced bool
	Group      string
	Kind       string
	Object     runtime.Object
}

var resources = []ResourceMeta{
	{
		Name:       "configmaps",
		Namespaced: true,
		Group:      "",
		Kind:       "ConfigMap",
		Object:     &core.ConfigMap{},
	},
	{
		Name:       "endpoints",
		Namespaced: true,
		Group:      "",
		Kind:       "Endpoints",
		Object:     &core.Endpoints{},
	},
	{
		Name:       "nodes",
		Namespaced: true,
		Group:      "",
		Kind:       "Node",
		Object:     &core.Node{},
	},
	{
		Name:       "pods",
		Namespaced: true,
		Group:      "",
		Kind:       "Pod",
		Object:     &core.Pod{},
	},
	{
		Name:       "secrets",
		Namespaced: true,
		Group:      "",
		Kind:       "Secret",
		Object:     &core.Secret{},
	},
	{
		Name:       "serviceaccounts",
		Namespaced: true,
		Group:      "",
		Kind:       "ServiceAccount",
		Object:     &core.ServiceAccount{},
	},
	{
		Name:       "service",
		Namespaced: true,
		Group:      "",
		Kind:       "Service",
		Object:     &core.Service{},
	},
	{
		Name:       "daemonsets",
		Namespaced: true,
		Group:      "apps",
		Kind:       "DaemonSet",
		Object:     &apps.DaemonSet{},
	},
	{
		Name:       "deployments",
		Namespaced: true,
		Group:      "apps",
		Kind:       "Deployment",
		Object:     &apps.Deployment{},
	},
	{
		Name:       "replicasets",
		Namespaced: true,
		Group:      "apps",
		Kind:       "ReplicaSet",
		Object:     &apps.ReplicaSet{},
	},
	{
		Name:       "statefulsets",
		Namespaced: true,
		Group:      "apps",
		Kind:       "StatefulSet",
		Object:     &apps.StatefulSet{},
	},
}

func getResourceMeta(resource string) (ResourceMeta, error) {
	for _, m := range resources {
		if m.Name == resource {
			return m, nil
		}
	}

	supportedResources := make([]string, len(resources))
	for i, m := range resources {
		supportedResources[i] = m.Kind
	}

	return ResourceMeta{}, fmt.Errorf("only support resources: %s", strings.Join(supportedResources, ","))
}

func restClient(clientset kubernetes.Interface, resource ResourceMeta) rest.Interface {
	switch resource.Group {
	case "":
		return clientset.CoreV1().RESTClient()
	case "apps":
		return clientset.AppsV1().RESTClient()
	}

	panic(fmt.Errorf("unsupport resource: %s", resource.Group))
}

func GetMeta(obj interface{}) (meta.TypeMeta, meta.ObjectMeta) {
	switch object := obj.(type) {
	case *apps.Deployment:
		return meta.TypeMeta{Kind: "Deployment"}, object.ObjectMeta
	case *apps.ReplicaSet:
		return meta.TypeMeta{Kind: "ReplicaSet"}, object.ObjectMeta
	case *apps.DaemonSet:
		return meta.TypeMeta{Kind: "DaemonSet"}, object.ObjectMeta

	case *core.Service:
		return meta.TypeMeta{Kind: "Service"}, object.ObjectMeta
	case *core.Pod:
		return meta.TypeMeta{Kind: "Pod"}, object.ObjectMeta

	case *core.Secret:
		return meta.TypeMeta{Kind: "Secret"}, object.ObjectMeta
	case *core.ConfigMap:
		return meta.TypeMeta{Kind: "ConfigMap"}, object.ObjectMeta

	case *core.ReplicationController:
		return meta.TypeMeta{Kind: "ReplicationController"}, object.ObjectMeta
	case *batch.Job:
		return meta.TypeMeta{Kind: "Job"}, object.ObjectMeta
	case *core.PersistentVolume:
		return meta.TypeMeta{Kind: "PersistentVolume"}, object.ObjectMeta
	case *core.Namespace:
		return meta.TypeMeta{Kind: "Namespace"}, object.ObjectMeta
	case *ext_v1beta1.Ingress:
		return meta.TypeMeta{Kind: "Ingress"}, object.ObjectMeta
	case *core.Node:
		return meta.TypeMeta{Kind: "Node"}, object.ObjectMeta
	case *rbac_v1beta1.ClusterRole:
		return meta.TypeMeta{Kind: "ClusterRole"}, object.ObjectMeta
	case *core.ServiceAccount:
		return meta.TypeMeta{Kind: "ServiceAccount"}, object.ObjectMeta
	}
	return meta.TypeMeta{}, meta.ObjectMeta{}
}
