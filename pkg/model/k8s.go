package model

import (
	"fmt"
	"time"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

type Event interface {
	GetIdentity() string

	GetCluster() string
	GetResourceVersion() string
	GetNamespace() string
	GetKind() string
	GetName() string

	GetAction() string
}

type EmbeddedEvent struct {
	Cluster         string `pg:",pk"`
	ResourceVersion string `pg:"type:integer,pk"`
	Namespace       string
	Kind            string
	Name            string
	Action          string `pg:"-"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `pg:",soft_delete"`
}

func (e *EmbeddedEvent) GetIdentity() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s", e.Cluster, e.Namespace, e.Kind, e.Name, e.ResourceVersion)
}
func (e *EmbeddedEvent) GetCluster() string         { return e.Cluster }
func (e *EmbeddedEvent) GetResourceVersion() string { return e.ResourceVersion }
func (e *EmbeddedEvent) GetNamespace() string       { return e.Namespace }
func (e *EmbeddedEvent) GetKind() string            { return e.Kind }
func (e *EmbeddedEvent) GetName() string            { return e.Name }
func (e *EmbeddedEvent) GetAction() string          { return e.Action }

type EventDeployment struct {
	*EmbeddedEvent
	Resource *apps.Deployment

	tableName struct{} `pg:"event"` //nolint
}

type EventConfigMap struct {
	*EmbeddedEvent
	Resource *core.ConfigMap

	tableName struct{} `pg:"event"` //nolint
}
