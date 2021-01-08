package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/j2gg0s/kubenotify/pkg/k8s"
	service "github.com/j2gg0s/kubenotify/pkg/service/kubenotify"
)

var (
	resources = []string{"deployments", "configmaps"}
	excludes  = []string{
		"metadata.*",
		"status.*",
	}
	extraExcludes = []string{}

	webhook = ""
)

func NewResourceController(
	resource string,
	kubeClient kubernetes.Interface,
	notifyFunc service.NotifyFunc,
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
		service.NewEventHandler(notifyFunc, excludes...),
	)

	return ctl, nil
}

func main() {
	cmd := cobra.Command{
		Use: "kubenotify",
	}

	cmd.PersistentFlags().StringSliceVar(&resources, "resources", resources, "The resource to watch")
	cmd.PersistentFlags().StringSliceVar(&excludes, "set-excludes", excludes, "The field should be exclude, clean and set")
	cmd.PersistentFlags().StringSliceVar(&extraExcludes, "add-excludes", extraExcludes, "The field should be exclude, add")
	cmd.PersistentFlags().StringVar(&webhook, "webhook", webhook, "The url of webhook")

	cmd.RunE = func(*cobra.Command, []string) error {
		if len(resources) == 0 {
			return fmt.Errorf("you should watch at least one resource")
		}
		excludes = append(excludes, extraExcludes...)

		rExcludes := make([]*regexp.Regexp, 0, len(excludes)+len(extraExcludes))
		for _, exclude := range append(excludes, extraExcludes...) {
			reg, err := regexp.Compile(exclude)
			if err != nil {
				return fmt.Errorf("compile regex %s with error: %w", exclude, err)
			}
			rExcludes = append(rExcludes, reg)
		}

		kubeClient := k8s.NewClient()
		notifyFunc := service.WebhookNotify(webhook)
		for _, resource := range resources {
			ctl, err := NewResourceController(
				resource, kubeClient, notifyFunc, rExcludes...)
			if err != nil {
				return err
			}
			stopCh := make(chan struct{})
			go ctl.Run(stopCh)
			defer close(stopCh)
		}

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM)
		signal.Notify(sigterm, syscall.SIGINT)
		<-sigterm

		return nil
	}

	if err := cmd.Execute(); err != nil {
		log.Err(err).Send()
	}
}
