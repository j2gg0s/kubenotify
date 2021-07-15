package workload

import (
	"fmt"

	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func NewStatefulSetController(
	podInformer coreinformers.PodInformer,
	ssInformer appsinformers.StatefulSetInformer,
	opts ...Option,
) (*controller, error) {
	o := newOptions().apply(opts...)

	ctl := newController(podInformer, o)

	ssLister := ssInformer.Lister()
	ctl.getFunc = func(key string) (interface{}, error) {
		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, fmt.Errorf("invalid deployment key(%s): %w", key, err)
		}

		ss, err := ssLister.StatefulSets(ns).Get(name)
		if err != nil {
			return nil, fmt.Errorf("get replicaset(%s/%s): %w", ns, name, err)
		}
		return ss, nil
	}

	ssInformer.Informer().AddEventHandler(newEventHandler(o))
	ssInformer.Informer().AddEventHandler(ctl)

	return ctl, nil
}
