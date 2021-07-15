package sentry

import (
	"fmt"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/util"
	"github.com/rs/zerolog/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func (ctl *Controller) Inspect(kind, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid %s key(%s): %w", kind, key, err)
	}

	if !ctl.hasSynced() {
		return ErrNotSynced
	}

	var owner types.UID
	var desired, ready int32
	var age time.Duration
	switch kind {
	case "Deployment":
		obj, err := ctl.dLister.Deployments(ns).Get(name)
		if err != nil {
			return fmt.Errorf("get deployment(%s): %w", key, err)
		}
		desired, ready = obj.Status.Replicas, obj.Status.ReadyReplicas

		if desired != ready {
			rsList, err := ctl.rsLister.ReplicaSets(ns).List(labels.Everything())
			if err != nil {
				return fmt.Errorf("list replicaset(%s): %w", ns, err)
			}
			replicaset := (*appsv1.ReplicaSet)(nil)
			for _, rs := range rsList {
				for _, owner := range rs.ObjectMeta.OwnerReferences {
					if owner.UID == obj.ObjectMeta.UID {
						if replicaset == nil || replicaset.ObjectMeta.CreationTimestamp.Time.Before(rs.ObjectMeta.CreationTimestamp.Time) {
							replicaset = rs
						}
					}
				}
			}
			if replicaset != nil {
				owner = replicaset.ObjectMeta.UID
				age = time.Since(replicaset.ObjectMeta.CreationTimestamp.Time)
			}
		}

	case "StatefulSet":
		obj, err := ctl.ssLister.StatefulSets(ns).Get(name)
		if err != nil {
			return fmt.Errorf("get statefulset(%s): %w", key, err)
		}
		desired, ready = obj.Status.Replicas, obj.Status.ReadyReplicas
		owner = obj.ObjectMeta.UID
		age = time.Since(obj.ObjectMeta.CreationTimestamp.Time)

		if desired != ready && ctl.EnableRevision {
			revision, err := ctl.crLister.ControllerRevisions(ns).Get(obj.Status.UpdateRevision)
			if err != nil {
				return fmt.Errorf("get revision(%s, %s): %w", ns, obj.Status.UpdateRevision, err)
			}
			age = time.Since(revision.ObjectMeta.CreationTimestamp.Time)
		}
	case "DaemonSet":
		obj, err := ctl.dsLister.DaemonSets(ns).Get(name)
		if err != nil {
			return fmt.Errorf("get daemonset(%s): %w", key, err)
		}
		desired, ready = obj.Status.DesiredNumberScheduled, obj.Status.NumberReady
		owner = obj.ObjectMeta.UID
		age = time.Since(obj.ObjectMeta.CreationTimestamp.Time)
		// TODO: daemonset changed at?
	}

	msg := fmt.Sprintf(
		"%s(%s) Age(%s) READY(%d/%d)",
		kind, key, util.PrettyDuration(age, 2), ready, desired)
	if desired == ready {
		log.Debug().Msgf(msg)
		return nil
	}

	pods, err := ctl.podLister.Pods(ns).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("list pods(%s): %w", ns, err)
	}
	for _, pod := range pods {
		isOwner := false
		for _, ref := range pod.ObjectMeta.OwnerReferences {
			if ref.UID == owner {
				isOwner = true
				break
			}
		}
		if !isOwner || pod.Status.Phase == corev1.PodRunning {
			continue
		}
		reason := pod.Status.Reason
		for _, cstatus := range pod.Status.ContainerStatuses {
			if cstatus.Ready {
				continue
			}

			if cstatus.State.Waiting != nil {
				reason = fmt.Sprintf("%s[%s]", cstatus.Name, cstatus.State.Waiting.Reason)
			} else if cstatus.State.Terminated != nil {
				reason = fmt.Sprintf("%s[%s]", cstatus.Name, cstatus.State.Terminated.Reason)
			}
		}

		msg = fmt.Sprintf("%s %s(%s)", msg, pod.Status.Phase, reason)
	}

	if err := ctl.notifyFunc(msg); err != nil {
		log.Warn().Err(err).Msgf("notify %s", msg)
	}

	return fmt.Errorf("%s(%s): %w", kind, key, ErrNotReady)
}

var (
	ErrNotReady  = fmt.Errorf("NotReady")
	ErrNotSynced = fmt.Errorf("NotSynced")
)
