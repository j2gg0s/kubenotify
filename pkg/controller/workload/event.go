package workload

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

type eventHandler struct {
	Options
}

func newEventHandler(opts Options) *eventHandler {
	return &eventHandler{opts}
}

var _ cache.ResourceEventHandler = (*eventHandler)(nil)

func (eh *eventHandler) OnAdd(obj interface{}) {
	eh.onAddOrDelete(obj, true)
}

func (eh *eventHandler) OnUpdate(pobj, nobj interface{}) {
	eh.onUpdate(pobj, nobj)
}

func (eh *eventHandler) OnDelete(obj interface{}) {
	eh.onAddOrDelete(obj, false)
}

func (eh *eventHandler) onAddOrDelete(obj interface{}, isAdd bool) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("get key of %T", obj)
		return
	}

	meta, err := metaapi.Accessor(obj)
	if err != nil {
		log.Warn().Err(err).Msgf("access meta of %T", obj)
		return
	}

	var st time.Time
	if isAdd {
		st = meta.GetCreationTimestamp().Time
	} else if v := meta.GetDeletionTimestamp(); v != nil {
		st = v.Time
	} else {
		st = time.Now()
	}

	if time.Since(st) > eh.IgnoreAfter {
		log.Debug().Msgf("ignore resource changed at %T", st)
		return
	}

	msg := fmt.Sprintf(
		"%s(%s) %s(%s) ResourceVersion(%s)",
		util.KindAccessor(obj), key,
		map[bool]string{true: "CreatedAt", false: "DeletedAt"}[isAdd],
		st.Format(time.RFC3339),
		meta.GetResourceVersion(),
	)

	if err := eh.NotifyFunc(msg); err != nil {
		log.Warn().Err(err).Msgf("notify")
	}
}

func (eh *eventHandler) onUpdate(pobj, nobj interface{}) {
	pmeta, err := metaapi.Accessor(pobj)
	if err != nil {
		log.Warn().Err(err).Msgf("get meta of %T", pobj)
		return
	}
	nmeta, err := metaapi.Accessor(nobj)
	if err != nil {
		log.Warn().Err(err).Msgf("get meta of %T", nobj)
		return
	}
	if pmeta.GetResourceVersion() == nmeta.GetResourceVersion() {
		log.Debug().Msgf("ignore resync")
		return
	}

	key, err := eh.KeyFunc(nobj)
	if err != nil {
		log.Warn().Err(err).Msgf("get key of %T", nobj)
	}
	kind := util.KindAccessor(nobj)
	msg := fmt.Sprintf(
		"%s(%s) ChangedAt(%s) Changes",
		kind, key, time.Now().Format(time.RFC3339))

	changes, err := diffObject(pobj, nobj)
	if err != nil {
		log.Warn().Err(err).Msgf("diff")
		return
	}
	changed := false
	for _, change := range changes {
		path := []byte(strings.Join(change.Path, "."))
		filter := false
		for _, re := range eh.Excludes {
			if re.Match(path) {
				filter = true
				break
			}
		}
		if filter {
			continue
		}
		changed = true
		msg = fmt.Sprintf("%s %s[%v->%v]", msg, path, change.From, change.To)
	}

	if !changed {
		log.Debug().Msgf("ignore update event without changes")
		return
	}

	if err := eh.NotifyFunc(msg); err != nil {
		log.Warn().Err(err).Msgf("notify update")
	}
}

func diffObject(x, y interface{}) ([]diff.Change, error) {
	mx, err := convertToMap(x)
	if err != nil {
		return nil, err
	}
	my, err := convertToMap(y)
	if err != nil {
		return nil, err
	}
	return diff.Diff(mx, my)
}

func convertToMap(obj interface{}) (map[string]interface{}, error) {
	d := map[string]interface{}{}
	if b, err := json.Marshal(obj); err != nil {
		return nil, fmt.Errorf("marshal with error: %w", err)
	} else if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("unmarshal with error: %w", err)
	}
	return d, nil
}
