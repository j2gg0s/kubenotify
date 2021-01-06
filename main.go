package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/j2gg0s/kubenotify/pkg/util"
	"github.com/r3labs/diff"
	"github.com/rs/zerolog/log"
)

var _ = cache.Controller(&Controller{})

type Controller struct{}

func (c *Controller) Run(stopCh <-chan struct{}) {
}

func (c *Controller) HasSynced() bool {
	return true
}

func (c *Controller) LastSyncResourceVersion() string {
	return ""
}

var (
	deployments = map[string]*apps.Deployment{}
)

func Run() {
	var kubeClient kubernetes.Interface
	if _, err := rest.InClusterConfig(); err != nil {
		kubeClient = util.GetClientOutOfCluster()
	} else {
		kubeClient = util.GetClient()
	}

	resource := &DeploymentResource{}

	_, ctl := cache.NewInformer(
		cache.NewListWatchFromClient(kubeClient.AppsV1().RESTClient(), "deployments", "", fields.Everything()),
		&apps.Deployment{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if err := Add(obj, resource); err != nil {
					log.Err(err).Send()
					return
				}
			},
			UpdateFunc: func(oldObj, obj interface{}) {
				if err := Add(obj, resource); err != nil {
					log.Err(err).Send()
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				if err := Del(obj, resource); err != nil {
					log.Err(err).Send()
					return
				}
			},
		},
	)

	stopCh := make(chan struct{})
	go ctl.Run(stopCh)
	defer close(stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func main() {
	Run()
}

type Resource interface {
	Key(obj interface{}) string
	Get(obj interface{}) (interface{}, bool)
	Set(obj interface{})
	Del(obj interface{})

	GetVersion(obj interface{}) int
	GetCreationTimestamp(obj interface{}) time.Time
	ShouldIgnore(path string) bool
}

type DeploymentResource struct {
}

func (r DeploymentResource) Key(obj interface{}) string {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("get name of resource with error: %v", obj)
		return ""
	}
	return key
}

func (r DeploymentResource) Get(obj interface{}) (interface{}, bool) {
	obj, ok := deployments[r.Key(obj)]
	if !ok {
		return nil, false
	}
	return obj, true
}

func (r DeploymentResource) Set(obj interface{}) {
	deployments[r.Key(obj)] = obj.(*apps.Deployment)
}

func (r DeploymentResource) Del(obj interface{}) {
	delete(deployments, r.Key(obj))
}

func (r DeploymentResource) GetVersion(obj interface{}) int {
	version := obj.(*apps.Deployment).ResourceVersion
	i, err := strconv.Atoi(version)
	if err != nil {
		log.Warn().Err(err).Msgf("convert version to int with error: %s", version)
		return -1
	}
	return i
}

func (r DeploymentResource) GetCreationTimestamp(obj interface{}) time.Time {
	return obj.(*apps.Deployment).CreationTimestamp.Time
}

var excludes = []string{
	"ObjectMeta.ResourceVersion",
	"ObjectMeta.Annotations.deployment.kubernetes.io/revision",
	"ObjectMeta.Generation",
	"Status.*",
}

func (r DeploymentResource) ShouldIgnore(path string) bool {
	for _, exclude := range excludes {
		if regexp.MustCompile(exclude).MatchString(path) {
			return true
		}
	}
	return false
}

func Add(obj interface{}, resource Resource) error {
	version := resource.GetVersion(obj)

	var left, right interface{}
	if v, ok := resource.Get(obj); ok {
		currVersion := resource.GetVersion(v)
		if currVersion < version {
			left, right = v, obj
		} else {
			left, right = obj, v
		}
	} else {
		left, right = nil, obj
	}
	resource.Set(obj)

	return Notify(left, right, resource)
}

func Del(obj interface{}, resource Resource) error {
	version := resource.GetVersion(obj)

	var left, right interface{}
	if v, ok := resource.Get(obj); ok {
		currVersion := resource.GetVersion(v)
		if currVersion < version {
			left, right = v, nil
		} else {
			left, right = obj, v
		}
	} else {
		left, right = obj, nil
	}
	if right == nil {
		resource.Del(right)
	}

	return Notify(left, right, resource)
}

func Notify(left, right interface{}, resource Resource) error {
	if left == nil {
		if time.Since(resource.GetCreationTimestamp(right)) > time.Minute {
			log.Debug().Msgf("ignore resource created before %s", "1m")
			return nil
		}
		msgf("resource create: %s", resource.Key(right))
		return nil
	}

	if right == nil {
		msgf("resource delete: %s", resource.Key(left))
		return nil
	}

	changes, err := diff.Diff(left, right)
	if err != nil {
		return fmt.Errorf("diff with error: %w", err)
	}

	notify := []diff.Change{}
	for _, change := range changes {
		paths := strings.Join(change.Path, ".")
		if resource.ShouldIgnore(paths) {
			continue
		}
		notify = append(notify, change)
	}

	if len(notify) > 0 {
		if b, err := json.MarshalIndent(notify, "", "  "); err != nil {
			return fmt.Errorf("json marshal with error: %w", err)
		} else {
			msgf("resource update: %s", string(b))
		}
	}
	return nil
}

func msgf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
