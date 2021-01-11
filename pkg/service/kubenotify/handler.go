package kubenotify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/r3labs/diff"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"k8s.io/client-go/tools/cache"

	core "k8s.io/api/core/v1"
)

type resourceEventHandler struct {
	store      cache.Store
	notifyFunc func(Event) error

	excludes []*regexp.Regexp
}

func NewEventHandler(notifyFunc func(Event) error, excludes ...*regexp.Regexp) cache.ResourceEventHandler {
	return resourceEventHandler{
		store: cache.NewStore(func(obj interface{}) (string, error) {
			return cache.MetaNamespaceKeyFunc(obj.(Resource).obj)
		}),
		notifyFunc: notifyFunc,
		excludes:   excludes,
	}
}

var _ cache.ResourceEventHandler = resourceEventHandler{}

func (h resourceEventHandler) OnAdd(obj interface{}) {
	r, err := NewResource(obj)
	if err != nil {
		log.Err(err).Msg("new resource with error")
		return
	}

	if err := h.add(*r); err != nil {
		log.Err(err).Msg("add resource with error")
		return
	}
}

func (h resourceEventHandler) OnUpdate(_, obj interface{}) {
	r, err := NewResource(obj)
	if err != nil {
		log.Err(err).Msg("new resource with error")
		return
	}

	if err := h.add(*r); err != nil {
		log.Err(err).Msg("add resource with error")
		return
	}
}

func (h resourceEventHandler) OnDelete(obj interface{}) {
	r, err := NewResource(obj)
	if err != nil {
		log.Err(err).Msg("new resource with error")
		return
	}

	if err := h.del(*r); err != nil {
		log.Err(err).Msg("add resource with error")
		return
	}
}

func (h resourceEventHandler) add(obj Resource) error {
	var left, right *Resource
	if v, ok, err := h.store.Get(obj); err != nil {
		return fmt.Errorf("get object[%s] from store with error: %w", obj.GetName(), err)
	} else if !ok {
		right = &obj
	} else {
		vv := v.(Resource)
		left = &vv
		right = &obj

		if !isLess(*left, *right) {
			left, right = right, left
		}
	}

	if err := h.store.Add(*right); err != nil {
		log.Err(err).Msgf("add object to store with error")
	}
	return h.notify(left, right)
}

func (h resourceEventHandler) del(obj Resource) error {
	var left, right *Resource
	if v, ok, err := h.store.Get(obj); err != nil {
		return fmt.Errorf("get object[%s] from store with error: %w", obj.GetName(), err)
	} else if !ok {
		left = &obj
	} else {
		vv := v.(Resource)
		left = &vv
		right = &obj

		if isLess(*left, *right) {
			// delete
			left = right
			right = nil
		} else {
			// new object had been created
			right = left
			left = nil
		}
	}

	if left != nil {
		if err := h.store.Delete(*left); err != nil {
			log.Err(err).Msgf("delete object from store with error")
		}
	}
	return h.notify(left, right)
}

func (h resourceEventHandler) notify(left, right *Resource) error {
	if left == nil {
		if time.Since(right.CreationTimestamp.Time) > time.Minute {
			log.Debug().Interface("create", right.obj).Msgf("ignore resoruce created before %s", "1m")
			return nil
		}
		return h.notifyFunc(newEvent("create", right))
	}

	if right == nil {
		return h.notifyFunc(newEvent("delete", left))
	}

	var err error
	var x, y map[string]interface{}
	switch right.obj.(type) {
	case core.ConfigMap:
		x, err = readConfigMap(left.obj.(core.ConfigMap))
		if err != nil {
			return err
		}
		y, err = readConfigMap(right.obj.(core.ConfigMap))
		if err != nil {
			return err
		}
	case *core.ConfigMap:
		xx := left.obj.(*core.ConfigMap)
		x, err = readConfigMap(*xx)
		if err != nil {
			return err
		}
		yy := right.obj.(*core.ConfigMap)
		y, err = readConfigMap(*yy)
		if err != nil {
			return err
		}
	default:
		x, err = ConvertToMap(left.obj)
		if err != nil {
			return err
		}
		y, err = ConvertToMap(right.obj)
		if err != nil {
			return err
		}
	}

	raw, err := diff.Diff(x, y)
	if err != nil {
		return fmt.Errorf("diff with error: %w", err)
	}
	changes := []diff.Change{}
	for _, change := range raw {
		if h.shouldIgnore(change) {
			continue
		}
		changes = append(changes, change)
	}
	if len(changes) == 0 {
		log.Debug().Interface("raw", raw).Msgf("ignore event[%s]", right.GetIdentify())
		return nil
	}

	return h.notifyFunc(newEvent("update", right, changes...))
}

func (h resourceEventHandler) shouldIgnore(change diff.Change) bool {
	for _, exclude := range h.excludes {
		if exclude.MatchString(strings.Join(change.Path, ".")) {
			return true
		}
	}
	return false
}

func readConfigMap(configMap core.ConfigMap) (map[string]interface{}, error) {
	cm, err := ConvertToMap(configMap)

	if err != nil {
		return nil, err
	}

	if _, ok := cm["data"]; !ok {
		return cm, nil
	}

	configs := map[string]interface{}{}
	for k, v := range cm["data"].(map[string]interface{}) {
		config, err := readConfig(k, v.(string))
		if err != nil {
			log.Warn().Err(err).Send()
			continue
		}
		configs[k] = config
	}
	cm["data"] = configs
	return cm, nil
}

func readConfig(name, value string) (map[string]interface{}, error) {
	v := viper.New()
	if ind := strings.LastIndex(name, "."); ind > 0 && ind < len(name)-1 {
		v.SetConfigType(name[ind+1:])
	}
	if err := v.ReadConfig(strings.NewReader(value)); err != nil {
		return nil, fmt.Errorf("unknown config type: %s", name)
	}
	data := map[string]interface{}{}
	for _, k := range v.AllKeys() {
		value := v.Get(k)
		if vv, ok := value.([]interface{}); ok {
			data[k] = castToJSONSlice(vv)
		} else {
			data[k] = value
		}
	}
	return data, nil
}

func castToMapStringInterface(src map[interface{}]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(src))
	for k, v := range src {
		dest[fmt.Sprintf("%v", k)] = v
	}
	return dest
}

func castToJSONSlice(src []interface{}) []interface{} {
	dest := make([]interface{}, len(src))
	for i, v := range src {
		switch vv := v.(type) {
		case map[interface{}]interface{}:
			dest[i] = castToMapStringInterface(vv)
		case []interface{}:
			dest[i] = castToJSONSlice(vv)
		default:
			dest[i] = v
		}
	}
	return dest
}

func ConvertToMap(obj interface{}) (map[string]interface{}, error) {
	d := map[string]interface{}{}
	if b, err := json.Marshal(obj); err != nil {
		return nil, fmt.Errorf("marshal with error: %w", err)
	} else if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("unmarshal with error: %w", err)
	}
	return d, nil
}
