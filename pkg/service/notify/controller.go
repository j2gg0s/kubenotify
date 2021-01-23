package notify

import (
	"fmt"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
)

func NewResourceController(
	resource string,
	kubeClient kubernetes.Interface,
	notifyFunc NotifyFunc,
	excludes ...*regexp.Regexp,
) (cache.Controller, error) {
	var restClient rest.Interface
	var object runtime.Object
	switch resource {
	case "configmaps":
		restClient = kubeClient.CoreV1().RESTClient()
		object = &core.ConfigMap{}
	case "deployments":
		restClient = kubeClient.AppsV1().RESTClient()
		object = &apps.Deployment{}
	case "daemonsets":
		restClient = kubeClient.AppsV1().RESTClient()
		object = &apps.DaemonSet{}
	case "statefulsets":
		restClient = kubeClient.AppsV1().RESTClient()
		object = &apps.StatefulSet{}
	default:
		return nil, fmt.Errorf("unsupport resource: %s", resource)
	}

	_, ctl := cache.NewInformer(
		cache.NewListWatchFromClient(restClient, resource, "", fields.Everything()),
		object,
		0,
		NewEventHandler(notifyFunc, excludes...),
	)

	return ctl, nil
}
