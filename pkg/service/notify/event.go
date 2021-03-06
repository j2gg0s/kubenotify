package notify

import (
	"github.com/r3labs/diff"
)

type Event struct {
	Meta Meta

	Action  string        `json:",omitempty"`
	Changes []diff.Change `json:",omitempty"`
}

type Meta struct {
	APIVersion string `json:",omitempty"`
	Kind       string `json:",omitempty"`

	Namespace       string `json:",omitempty"`
	Name            string `json:",omitempty"`
	ResourceVersion string `json:",omitempty"`
	SelfLink        string `json:",omitempty"`
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
			SelfLink:        obj.SelfLink,
		},
	}
}
