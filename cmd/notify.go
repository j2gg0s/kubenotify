package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/j2gg0s/kubenotify/pkg/controller/workload"
	"github.com/j2gg0s/kubenotify/pkg/notify"
	"github.com/spf13/cobra"
	"k8s.io/client-go/informers"
)

var (
	resources = []string{"deployments", "configmaps", "daemonsets", "statefulsets"}
	excludes  = []string{
		"metadata.*",
		"status.*",
	}
	extraExcludes  = []string{}
	webhooks       = []string{}
	ignoreAfterRaw = "15m"
)

func NewNotifyCommand() *cobra.Command {
	root := cobra.Command{
		Use: "notify",
	}

	root.PersistentFlags().StringSliceVar(
		&resources,
		"resources",
		resources,
		"The resource to watch")
	root.PersistentFlags().StringSliceVar(
		&excludes,
		"set-excludes",
		excludes,
		"The field should be exclude, clean and set")
	root.PersistentFlags().StringSliceVar(
		&extraExcludes,
		"add-excludes",
		extraExcludes,
		"The field should be exclude, add")
	root.PersistentFlags().StringSliceVar(
		&webhooks,
		"webhooks",
		webhooks,
		"The urls of webhook")
	root.PersistentFlags().StringVar(
		&ignoreAfterRaw,
		"ignore-after",
		ignoreAfterRaw,
		"Ignore after resource create, support unit ns, us, ms, s, m, h",
	)

	root.RunE = func(cmd *cobra.Command, args []string) error {
		opts := []workload.Option{}

		if len(resources) == 0 {
			return fmt.Errorf("you should watch at least one resource")
		}
		excludes = append(excludes, extraExcludes...)

		rExcludes := make([]*regexp.Regexp, 0, len(excludes)+len(extraExcludes))
		for _, exclude := range append(excludes, extraExcludes...) {
			reg, err := regexp.Compile(exclude)
			if err != nil {
				return fmt.Errorf("compile regex %s with error: %w", exclude, err)
			}
			rExcludes = append(rExcludes, reg)
		}
		if len(rExcludes) > 0 {
			opts = append(opts, workload.WithExcludes(rExcludes))
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		ignoreAfter, err := time.ParseDuration(ignoreAfterRaw)
		if err != nil {
			return fmt.Errorf("invalid duration %s: %w", ignoreAfterRaw, err)
		}
		opts = append(opts, workload.WithIgnoreAfter(ignoreAfter))

		informer := informers.NewSharedInformerFactory(kubeClient, time.Minute)

		notifyFunc := notify.StdoutNotify()
		if len(webhooks) > 0 {
			notifyFunc = notify.WebhooksNotify(webhooks)
		}
		opts = append(opts, workload.WithNotifyFunc(notifyFunc))

		podInformer := informer.Core().V1().Pods()
		{
			dc, err := workload.NewDeploymentController(
				podInformer,
				informer.Apps().V1().ReplicaSets(),
				informer.Apps().V1().Deployments(),
				opts...,
			)
			if err != nil {
				return err
			}
			go dc.Run(1, ctx.Done())
		}
		{
			ssc, err := workload.NewStatefulSetController(
				podInformer,
				informer.Apps().V1().StatefulSets(),
				opts...,
			)
			if err != nil {
				return err
			}
			go ssc.Run(1, ctx.Done())
		}

		go informer.Start(ctx.Done())

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM)
		signal.Notify(sigterm, syscall.SIGINT)
		<-sigterm

		return nil
	}

	return &root
}
