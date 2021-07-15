package workload

import (
	"fmt"

	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func NewDeploymentController(
	podInformer coreinformers.PodInformer,
	rsInformer appsinformers.ReplicaSetInformer,
	dInformer appsinformers.DeploymentInformer,
	opts ...Option,
) (*controller, error) {
	o := newOptions().apply(opts...)

	ctl := newController(podInformer, o)

	rsLister := rsInformer.Lister()
	ctl.getFunc = func(key string) (interface{}, error) {
		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, fmt.Errorf("invalid deployment key(%s): %w", key, err)
		}

		rs, err := rsLister.ReplicaSets(ns).Get(name)
		if err != nil {
			return nil, fmt.Errorf("get replicaset(%s/%s): %w", ns, name, err)
		}
		return rs, nil
	}

	dInformer.Informer().AddEventHandler(newEventHandler(o))
	rsInformer.Informer().AddEventHandler(ctl)

	return ctl, nil
}
