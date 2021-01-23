package sync

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Event struct {
	Key       string
	EventType string
	From      interface{}
	To        interface{}
}

type Controller struct {
	client   kubernetes.Interface
	informer cache.SharedInformer
	queue    workqueue.RateLimitingInterface
	logger   zerolog.Logger
	handler  func(Event) error
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Info().Msg("start kwatch watcher")
	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	c.logger.Info().Msg("kwatch controller synced and ready")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// HasSynced is required for the cache.Controller interface.
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is required for the cache.Controller interface.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

const maxRetries = 5

func (c *Controller) processNextItem() bool {
	v, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(v)

	event := v.(Event)
	err := c.handler(event)
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(event)
		return true
	}

	key := ""
	if event.EventType == "delete" {
		key, _ = cache.MetaNamespaceKeyFunc(event.From)
	} else {
		key, _ = cache.MetaNamespaceKeyFunc(event.To)
	}

	if c.queue.NumRequeues(event) < maxRetries {
		c.logger.Warn().Err(err).Msgf("Error processing event and retry: %v", key)
		c.queue.AddRateLimited(event)
	} else {
		// err != nil and too many retries
		c.logger.Error().Err(err).Msgf("Error processing event and giving up: %s", key)
		c.queue.Forget(event)
		utilruntime.HandleError(err)
	}

	return true
}

func NewController(
	namespace, resource string,
	clientset kubernetes.Interface, handler func(Event) error,
	logger zerolog.Logger,
) (*Controller, error) {
	resourceMeta, err := getResourceMeta(resource)
	if err != nil {
		return nil, err
	}

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := cache.NewSharedInformer(
		cache.NewListWatchFromClient(
			restClient(clientset, resourceMeta), resource, namespace, fields.Everything()),
		resourceMeta.Object,
		0,
	)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			queue.Add(Event{
				EventType: "create",
				To:        obj,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			queue.Add(Event{
				EventType: "update",
				From:      oldObj,
				To:        newObj,
			})
		},
		DeleteFunc: func(obj interface{}) {
			queue.Add(Event{
				EventType: "delete",
				From:      obj,
			})
		},
	})

	return &Controller{
		client:   clientset,
		informer: informer,
		queue:    queue,
		logger:   logger,
		handler:  handler,
	}, nil
}
