package sentry

import (
	"strings"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/notify"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	*Options

	notifyFunc notify.NotifyFunc

	podLister corelisters.PodLister
	rsLister  appslisters.ReplicaSetLister
	dLister   appslisters.DeploymentLister
	ssLister  appslisters.StatefulSetLister
	dsLister  appslisters.DaemonSetLister

	crLister appslisters.ControllerRevisionLister

	hasSynced func() bool

	queue workqueue.RateLimitingInterface
}

func New(
	podInformer coreinformers.PodInformer,
	rsInformer appsinformers.ReplicaSetInformer,
	dInformer appsinformers.DeploymentInformer,
	ssInformer appsinformers.StatefulSetInformer,
	dsInformer appsinformers.DaemonSetInformer,
	crInformer appsinformers.ControllerRevisionInformer,
	notifyFunc notify.NotifyFunc,
	opts ...Option,
) (*Controller, error) {
	options := newOptions()
	for _, opt := range opts {
		opt(options)
	}
	ctl := Controller{
		Options: options,

		notifyFunc: notifyFunc,

		podLister: podInformer.Lister(),
		rsLister:  rsInformer.Lister(),
		dLister:   dInformer.Lister(),
		ssLister:  ssInformer.Lister(),
		dsLister:  dsInformer.Lister(),

		hasSynced: podInformer.Informer().HasSynced,

		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(
					options.InitBackoff, options.MaxBackoff,
				),
				&workqueue.BucketRateLimiter{
					Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			"kubenotify-controller",
		),
	}

	// watch pod & replicaset
	_ = podInformer.Informer()
	_ = rsInformer.Informer()
	if ctl.EnableRevision {
		ctl.crLister = crInformer.Lister()
		_ = crInformer.Informer()
	}

	if len(options.IncludeResources) == 0 || options.IncludeResources["Deployment"] {
		dInformer.Informer().AddEventHandler(&ctl)
	}
	if len(options.IncludeResources) == 0 || options.IncludeResources["StatefulSet"] {
		ssInformer.Informer().AddEventHandler(&ctl)
	}
	if len(options.IncludeResources) == 0 || options.IncludeResources["DaemonSet"] {
		dsInformer.Informer().AddEventHandler(&ctl)
	}

	return &ctl, nil
}

func (ctl *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer ctl.queue.ShutDown()

	for i := 0; i < workers; i++ {
		go wait.Until(ctl.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (ctl *Controller) worker() {
	for ctl.processNextWorkItem() {
	}
}

func (ctl *Controller) processNextWorkItem() bool {
	raw, quit := ctl.queue.Get()
	if quit {
		return false
	}
	defer ctl.queue.Done(raw)

	keys := strings.SplitN(raw.(string), ";", 2)
	if len(keys) != 2 {
		log.Warn().Msgf("invalid key: %s", raw.(string))
		return true
	}

	err := ctl.Inspect(keys[0], keys[1])
	ctl.handleErr(err, raw)

	return true
}

func (ctl *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		ctl.queue.Forget(key)
		return
	}

	_, _, kerr := cache.SplitMetaNamespaceKey(key.(string))
	if kerr != nil {
		log.Error().Err(err).Msgf("Failed to split meta namespace cache key(%s)", key)
	}

	if ctl.queue.NumRequeues(key) < ctl.MaxRetries {
		ctl.queue.AddRateLimited(key)
		return
	}

	runtime.HandleError(err)

	ctl.queue.Forget(key)
}
