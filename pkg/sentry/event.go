package sentry

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/util"
	"github.com/r3labs/diff"
	"github.com/rs/zerolog/log"
	metaapi "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

var _ cache.ResourceEventHandler = (*Controller)(nil)

func (ctl *Controller) OnAdd(obj interface{}) {
	ctl.onChange(nil, obj)
}

func (ctl *Controller) OnDelete(obj interface{}) {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = tombstone.Obj
	}
	ctl.onChange(obj, nil)
}

func (ctl *Controller) OnUpdate(before, after interface{}) {
	ctl.onChange(before, after)
}

func (ctl *Controller) onChange(before, after interface{}) {
	obj := after
	if obj == nil {
		obj = before
	}

	key, err := ctl.KeyFunc(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("acces key of %T", obj)
		return
	}

	kind := util.KindAccessor(obj)

	meta, err := metaapi.Accessor(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("access meta of %T", obj)
		return
	}

	if before == nil &&
		time.Since(meta.GetCreationTimestamp().Time) > ctl.IgnoreCreatedBefore {

		// NOTE: when restart
		log.Debug().Msgf("ignore %T(%s): create before %v", obj, key, ctl.IgnoreCreatedBefore)
		return
	}

	if len(ctl.IncludeNamespaces) > 0 &&
		meta.GetNamespace() != "" &&
		!ctl.IncludeNamespaces[meta.GetNamespace()] {

		log.Debug().Msgf(
			"ignore %T(%s): namespace %s",
			obj, key,
			meta.GetNamespace())
		return
	}

	var msg string
	if before == nil {
		msg = fmt.Sprintf(
			"%s(%s) CreatedAt(%s)",
			kind, key,
			meta.GetCreationTimestamp().Format(ctl.TimeFormat))
	} else if after == nil {
		msg = fmt.Sprintf(
			"%s(%s) DeletedAt(%s)",
			kind, key,
			time.Now().Format(ctl.TimeFormat))
	} else {
		changes, err := diffAsMap(before, after)
		if err != nil {
			log.Warn().Err(err).Msgf("diff")
			return
		}
		msgs := []string{fmt.Sprintf("%s(%s) ChangedAt(%s)", kind, key, time.Now().Format(ctl.TimeFormat))}
		for _, change := range changes {
			path := []byte(strings.Join(change.Path, "."))

			if len(ctl.Excludes) > 0 {
				exclude := false
				for _, regex := range ctl.Excludes {
					if regex.Match(path) {
						exclude = true
						break
					}
				}
				if exclude {
					continue
				}
			}

			if len(ctl.Includes) > 0 {
				include := false
				for _, regex := range ctl.Includes {
					if regex.Match(path) {
						include = true
						break
					}
				}
				if !include {
					continue
				}
			}

			msgs = append(
				msgs,
				fmt.Sprintf("%s(%v - %v)", string(path), change.From, change.To))
		}
		if len(msgs) == 1 {
			log.Debug().Msgf("ignore %s(%s-%s)", kind, key, meta.GetResourceVersion())
			return
		}

		msg = strings.Join(msgs, " ")
	}

	log.Debug().Msgf("enqueue %s(%s-%s)", kind, key, meta.GetResourceVersion())
	ctl.queue.Add(fmt.Sprintf("%s;%s", kind, key))

	if ctl.Debug {
		msg = fmt.Sprintf("%s ResourceVersion(%s)", msg, meta.GetResourceVersion())
	}

	if err := ctl.notifyFunc(msg); err != nil {
		log.Warn().Err(err).Msgf("notify msg(%s)", msg)
	}
}

func diffAsMap(before, after interface{}) ([]diff.Change, error) {
	bm, err := convertToMap(before)
	if err != nil {
		return nil, fmt.Errorf("convert %T to map: %w", before, err)
	}
	am, err := convertToMap(after)
	if err != nil {
		return nil, fmt.Errorf("convert %T to map: %w", after, err)
	}
	changes, err := diff.Diff(bm, am)
	if err != nil {
		return nil, fmt.Errorf("diff %T vs %T: %w", before, after, err)
	}
	return changes, nil
}

func convertToMap(obj interface{}) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	if b, err := json.Marshal(obj); err != nil {
		return nil, err
	} else if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
