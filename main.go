package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/client"
	"github.com/j2gg0s/kubenotify/pkg/notify"
	"github.com/j2gg0s/kubenotify/pkg/sentry"
	"github.com/spf13/cobra"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var (
	debug        = false
	outofCluster = false
	kubeClient   kubernetes.Interface

	excludes = []string{
		`metadata\.[acdfgmors].*`,
		`status\..*`,
		`spec\.template\.spec\.containers\.[123456789]`,
		// jaeger injected
		`metadata\.labels\.sidecar\.jaegertracing\.io\/injected`,
	}
	includes          = []string{}
	webhooks          = []string{}
	ignoreBefore      = "1m"
	includeResources  = []string{}
	includeNamespaces = []string{}
	resync            = "1m"
	disableRevision   = true
)

func main() {
	root := cobra.Command{
		Use:  "kubenotify",
		Long: "subscribe kubernetes workload change event, support Deployment, StatefulSet and DaemonSet",
	}

	root.PersistentFlags().BoolVar(&debug, "debug", debug, "enable debug log")
	root.PersistentFlags().BoolVar(&disableRevision, "disable-revision", disableRevision, "disable revision")
	root.PersistentFlags().BoolVar(&outofCluster, "outof-cluster", outofCluster, "use outof cluster config directly")
	root.PersistentFlags().StringVar(&ignoreBefore, "ignore-before", ignoreBefore, "ignore create before when start")
	root.PersistentFlags().StringSliceVar(&excludes, "excludes", excludes, "excludes resource field when diff")
	root.PersistentFlags().StringSliceVar(&includes, "includes", includes, "only include resource field when diff")
	root.PersistentFlags().StringSliceVar(&includeResources, "resources", includeResources, "watch only these resource, default all, support Deployment, StatefulSet, DaemonSet")
	root.PersistentFlags().StringSliceVar(&includeNamespaces, "namespaces", includeNamespaces, "watch resource under these namepsace, default all")
	root.PersistentFlags().StringVar(&resync, "resync", resync, "duration to resync resource")
	root.PersistentFlags().StringSliceVar(&webhooks, "webhooks", webhooks, "webhook to notify")

	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}

		var err error
		kubeClient, err = client.NewKubeClient(outofCluster)
		if err != nil {
			return err
		}

		return nil
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		var err error
		opts := []sentry.Option{}

		if debug {
			opts = append(opts, sentry.EnableDebug())
		}
		if disableRevision {
			opts = append(opts, sentry.DisableRevision())
		}

		if len(excludes) > 0 {
			rExcludes := make([]*regexp.Regexp, 0, len(excludes))
			for _, exclude := range excludes {
				reg, err := regexp.Compile(exclude)
				if err != nil {
					return fmt.Errorf("compile regex %s: %w", exclude, err)
				}
				rExcludes = append(rExcludes, reg)
			}
			opts = append(opts, sentry.WithExcludes(rExcludes))
		}

		if len(includes) > 0 {
			rIncludes := make([]*regexp.Regexp, 0, len(includes))
			for _, include := range includes {
				reg, err := regexp.Compile(include)
				if err != nil {
					return fmt.Errorf("compile regex %s: %w", include, err)
				}
				rIncludes = append(rIncludes, reg)
			}
			opts = append(opts, sentry.WithIncludes(rIncludes))
		}

		if len(includeResources) > 0 {
			opts = append(opts, sentry.IncludeResources(includeResources...))
		}
		if len(includeNamespaces) > 0 {
			opts = append(opts, sentry.IncludeNamespaces(includeNamespaces...))
		}

		d, err := time.ParseDuration(ignoreBefore)
		if err != nil {
			return fmt.Errorf("parse duration %s: %w", ignoreBefore, err)
		}
		opts = append(opts, sentry.WithIgnoreCreatedBefore(d))

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		notifyFunc := notify.StdoutNotify()
		if len(webhooks) > 0 {
			notifyFunc = notify.WebhooksNotify(webhooks)
		}

		var informer informers.SharedInformerFactory
		{
			d, err := time.ParseDuration(resync)
			if err != nil {
				return fmt.Errorf("prase duration %s: %w", resync, err)
			}
			informer = informers.NewSharedInformerFactory(kubeClient, d)
		}

		ctl, err := sentry.New(
			informer.Core().V1().Pods(),
			informer.Apps().V1().ReplicaSets(),
			informer.Apps().V1().Deployments(),
			informer.Apps().V1().StatefulSets(),
			informer.Apps().V1().DaemonSets(),
			informer.Apps().V1().ControllerRevisions(),
			notifyFunc,
			opts...,
		)
		if err != nil {
			return err
		}

		go ctl.Run(1, ctx.Done())
		go informer.Start(ctx.Done())

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM)
		signal.Notify(sigterm, syscall.SIGINT)
		<-sigterm

		return nil
	}

	if err := root.Execute(); err != nil {
		log.Err(err).Send()
	}
}
