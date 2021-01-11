package kubenotify

import (
	"fmt"
	"strconv"

	"github.com/rs/zerolog/log"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

type Resource struct {
	obj interface{}

	meta.TypeMeta
	meta.ObjectMeta
}

func (r Resource) GetIdentify() string {
	return fmt.Sprintf(
		"%s, Kind=%s, %s/%s, ResourceVersion=%s",
		r.APIVersion, r.Kind,
		r.Namespace, r.Name,
		r.ResourceVersion,
	)
}

func NewResource(obj interface{}) (*Resource, error) {
	r := &Resource{
		obj: obj,
	}

	var typeMeta meta.TypeMeta
	var objectMeta meta.ObjectMeta
	switch v := obj.(type) {
	case core.ConfigMap:
		typeMeta = v.TypeMeta
		objectMeta = v.ObjectMeta
	case *core.ConfigMap:
		typeMeta = v.TypeMeta
		objectMeta = v.ObjectMeta
	case apps.Deployment:
		typeMeta = v.TypeMeta
		objectMeta = v.ObjectMeta
	case *apps.Deployment:
		typeMeta = v.TypeMeta
		objectMeta = v.ObjectMeta
	}

	r.TypeMeta = typeMeta
	r.ObjectMeta = objectMeta

	return r, nil
}

func isLess(left, right Resource) bool {
	// NOTE: 什么情况下会没有
	if left.ResourceVersion == "" || right.ResourceVersion == "" {
		return false
	}

	x, err := strconv.Atoi(left.GetResourceVersion())
	if err != nil {
		log.Warn().Err(err).Msgf("convert ResourceVersion[%s] to int with error", left.GetResourceVersion())
	}

	y, err := strconv.Atoi(right.GetResourceVersion())
	if err != nil {
		log.Warn().Err(err).Msgf("convert ResourceVersion[%s] to int with error", right.GetResourceVersion())
	}

	return x < y
}
