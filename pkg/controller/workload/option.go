package workload

import (
	"regexp"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/notify"
	"k8s.io/client-go/tools/cache"
)

type Options struct {
	KeyFunc     func(interface{}) (string, error)
	IgnoreAfter time.Duration
	NotifyFunc  notify.NotifyFunc
	Excludes    []*regexp.Regexp
}

func newOptions() Options {
	return Options{
		KeyFunc:     cache.DeletionHandlingMetaNamespaceKeyFunc,
		IgnoreAfter: time.Minute * 5,
	}
}

func (o Options) apply(opts ...Option) Options {
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

type Option func(*Options)

func WithKeyFunc(keyFunc func(interface{}) (string, error)) Option {
	return func(o *Options) {
		o.KeyFunc = keyFunc
	}
}

func WithIgnoreAfter(ignoreAfter time.Duration) Option {
	return func(o *Options) {
		o.IgnoreAfter = ignoreAfter
	}
}

func WithNotifyFunc(notifyFunc notify.NotifyFunc) Option {
	return func(o *Options) {
		o.NotifyFunc = notifyFunc
	}
}

func WithExcludes(excludes []*regexp.Regexp) Option {
	return func(o *Options) {
		o.Excludes = excludes
	}
}
