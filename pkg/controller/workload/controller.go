package workload

import (
	"fmt"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/util"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	metaapi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var maxRetries = 15

type controller struct {
	Options

	getFunc func(string) (interface{}, error)

	podLister    corelisters.PodLister
	podHasSynced func() bool

	readyQueue workqueue.RateLimitingInterface

	succeed cache.Store
}

func newController(
	podInformer coreinformers.PodInformer,
	opts Options,
) *controller {
	return &controller{
		Options:      opts,
		podLister:    podInformer.Lister(),
		podHasSynced: podInformer.Informer().HasSynced,
		readyQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.NewMaxOfRateLimiter(
				// 1s, 2s, 4s, 8s, 16s, 32s, 1m4s, 2m8s, 4m16s, 8m32s
				workqueue.NewItemExponentialFailureRateLimiter(
					time.Second, opts.IgnoreAfter,
				),
				&workqueue.BucketRateLimiter{
					Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			"kubenotify-controller",
		),
		succeed: cache.NewTTLStore(func(obj interface{}) (string, error) {
			meta, ok := obj.(metav1.Object)
			if !ok {
				return "", fmt.Errorf("object is not ObjectMeta: %T", meta)
			}
			if len(meta.GetNamespace()) > 0 {
				return meta.GetNamespace() + "/" + meta.GetName(), nil
			}
			return meta.GetName(), nil
		}, opts.IgnoreAfter),
	}
}

/* k8s controller pattern */
func (ctl *controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ctl.readyQueue.ShutDown()

	for i := 0; i < workers; i++ {
		// TODO
		go wait.Until(ctl.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (ctl *controller) worker() {
	for ctl.processNextWorkItem() {
	}
}

func (ctl *controller) processNextWorkItem() bool {
	key, quit := ctl.readyQueue.Get()
	if quit {
		return false
	}
	defer ctl.readyQueue.Done(key)

	err := ctl.handle(key.(string))
	ctl.handleErr(err, key)

	return true
}

func (ctl *controller) handle(key string) (err error) {
	obj, err := ctl.getFunc(key)
	if err != nil {
		return fmt.Errorf("get resource from %s: %w", key, err)
	}

	kind := util.KindAccessor(obj)
	replicas, err := util.ReplicasAccessor(obj)
	if err != nil {
		return fmt.Errorf("access replicas of %T", obj)
	}
	meta, err := metaapi.Accessor(obj)
	if err != nil {
		return fmt.Errorf("access meta of %T", obj)
	}

	if meta.GetDeletionTimestamp() != nil ||
		replicas == 0 ||
		time.Since(meta.GetCreationTimestamp().Time) > ctl.IgnoreAfter {

		log.Debug().Msgf("ignore resource(%s/%s)", meta.GetNamespace(), meta.GetName())
		return nil
	}

	if !ctl.podHasSynced() {
		return fmt.Errorf("pod has not synced")
	}

	pods, err := ctl.podLister.Pods(meta.GetNamespace()).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	running, pending, otherPods := int32(0), int32(0), []*corev1.Pod{}
	for _, pod := range pods {
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.UID == meta.GetUID() {
				switch pod.Status.Phase {
				case corev1.PodRunning:
					running += 1
				case corev1.PodPending:
					pending += 1
				default:
					otherPods = append(otherPods, pod)
				}
			}
		}
	}

	msg := fmt.Sprintf(
		"%s(%s) Age(%s) Replicas(%d)",
		kind, key,
		util.PrettyDuration(time.Since(meta.GetCreationTimestamp().Time), 2),
		replicas,
	)
	if running != 0 {
		msg = fmt.Sprintf("%s Running(%d)", msg, running)
	}
	if pending != 0 {
		msg = fmt.Sprintf("%s Pending(%d)", msg, pending)
	}
	if len(otherPods) > 0 {
		msg = fmt.Sprintf("%s Other(%d)", msg, len(otherPods))
		for _, pod := range otherPods {
			msg = fmt.Sprintf("%s [%s]", msg, pod.Status.Message)
		}
	}

	if err := ctl.NotifyFunc(msg); err != nil {
		return fmt.Errorf("send notify: %w", err)
	}

	if running == replicas {
		return nil
	}

	return fmt.Errorf("resource is not ready")
}

func (ctl *controller) handleErr(err error, key interface{}) {
	if err == nil {
		ctl.readyQueue.Forget(key)
		return
	}

	_, _, kerr := cache.SplitMetaNamespaceKey(key.(string))
	if kerr != nil {
		log.Error().Err(err).Msgf("Failed to split meta namespace cache key(%s)", key)
	}

	if ctl.readyQueue.NumRequeues(key) < maxRetries {
		ctl.readyQueue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)

	ctl.readyQueue.Forget(key)
}

/* resource event handler */
func (ctl *controller) OnAdd(obj interface{}) {
	ctl.enqueue(obj)
}

func (ctl *controller) OnUpdate(pobj, obj interface{}) {
	// resync
	pmeta, err := metaapi.Accessor(pobj)
	if err != nil {
		log.Warn().Err(err).Msgf("get meta of %T", pobj)
		return
	}
	meta, err := metaapi.Accessor(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("get meta of %T", obj)
		return
	}
	if pmeta.GetResourceVersion() == meta.GetResourceVersion() {
		return
	}

	ctl.enqueue(obj)
}

func (ctl *controller) OnDelete(obj interface{}) {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = tombstone.Obj
	}
	ctl.readyQueue.Done(obj)
}

func (ctl *controller) enqueue(obj interface{}) {
	key, err := ctl.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(
			fmt.Errorf("coludn't get key of object %#v: %w", obj, err))
		return
	}
	ctl.readyQueue.Add(key)
}
