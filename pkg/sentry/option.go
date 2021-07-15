package sentry

import (
	"regexp"
	"time"

	"k8s.io/client-go/tools/cache"
)

type Options struct {
	KeyFunc func(interface{}) (string, error)

	TimeFormat string

	InitBackoff time.Duration
	MaxBackoff  time.Duration
	MaxRetries  int

	IgnoreCreatedBefore time.Duration

	Excludes []*regexp.Regexp
	Includes []*regexp.Regexp

	// Namespaces, watch only these namespaces, default all
	IncludeNamespaces map[string]bool
	// Resources, watch only these resources, default all
	// Support Deployment, StatefulSet, DaemonSet
	IncludeResources map[string]bool

	Debug          bool
	EnableRevision bool
}

type Option func(*Options)

func newOptions() *Options {
	return &Options{
		KeyFunc:    cache.DeletionHandlingMetaNamespaceKeyFunc,
		TimeFormat: "15:04:05Z07:00",

		// 1s, 2s, 4s, 8s, 16s, 32s, 1m4s, 2m8s, 4m16s, 8m32s
		InitBackoff: time.Second,
		MaxBackoff:  time.Minute * 5,
		MaxRetries:  10,

		IgnoreCreatedBefore: time.Minute,

		EnableRevision: true,
	}
}

func IncludeResources(resources ...string) Option {
	return func(o *Options) {
		if o.IncludeResources == nil {
			o.IncludeResources = map[string]bool{}
		}

		for _, resource := range resources {
			o.IncludeResources[resource] = true
		}
	}
}

func IncludeNamespaces(namespaces ...string) Option {
	return func(o *Options) {
		if o.IncludeNamespaces == nil {
			o.IncludeNamespaces = map[string]bool{}
		}

		for _, ns := range namespaces {
			o.IncludeNamespaces[ns] = true
		}
	}
}

func WithKeyFunc(kf func(interface{}) (string, error)) Option {
	return func(o *Options) {
		o.KeyFunc = kf
	}
}

func WithTimeFormat(tf string) Option {
	return func(o *Options) {
		o.TimeFormat = tf
	}
}

func WithExcludes(excludes []*regexp.Regexp) Option {
	return func(o *Options) {
		o.Excludes = excludes
	}
}

func WithIncludes(includes []*regexp.Regexp) Option {
	return func(o *Options) {
		o.Includes = includes
	}
}

func WithIgnoreCreatedBefore(d time.Duration) Option {
	return func(o *Options) {
		o.IgnoreCreatedBefore = d
	}
}

func EnableDebug() Option {
	return func(o *Options) {
		o.Debug = true
	}
}

func DisableRevision() Option {
	return func(o *Options) {
		o.EnableRevision = false
	}
}
