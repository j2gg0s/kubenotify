package kubenotify

import (
	"github.com/r3labs/diff"
)

type Event struct {
	Name string
	Meta Meta

	Action  string
	Changes []diff.Change
}

type Meta struct {
	APIVersion string
	Kind       string

	Namespace       string
	Name            string
	ResourceVersion string
}

func newEvent(action string, obj *Resource, changes ...diff.Change) Event {
	return Event{
		Action:  action,
		Changes: changes,

		Meta: Meta{
			APIVersion: obj.APIVersion,
			Kind:       obj.Kind,

			Namespace:       obj.Namespace,
			Name:            obj.Name,
			ResourceVersion: obj.ResourceVersion,
		},
	}
}
