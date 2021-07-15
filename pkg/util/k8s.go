package util

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

func PodTemplateAccessor(obj interface{}) (core.PodSpec, error) {
	switch v := obj.(type) {
	case *apps.Deployment:
		return v.Spec.Template.Spec, nil
	case *apps.StatefulSet:
		return v.Spec.Template.Spec, nil
	}
	return core.PodSpec{}, fmt.Errorf("unknown type: %T", obj)
}

func ReplicasAccessor(obj interface{}) (int32, error) {
	switch v := obj.(type) {
	case *apps.Deployment:
		return *v.Spec.Replicas, nil
	case *apps.ReplicaSet:
		return *v.Spec.Replicas, nil
	case *apps.StatefulSet:
		return *v.Spec.Replicas, nil
	}
	return 0, fmt.Errorf("unknown type: %T", obj)
}

func KindAccessor(obj interface{}) string {
	switch obj.(type) {
	case *apps.Deployment:
		return "Deployment"
	case *apps.ReplicaSet:
		return "ReplicaSet"
	case *apps.StatefulSet:
		return "StatefulSet"
	}
	return ""
}
