package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	service "github.com/j2gg0s/kubenotify/pkg/service/notify"
	"github.com/spf13/cobra"
)

var (
	resources = []string{"deployments", "configmaps", "daemonsets", "statefulsets"}
	excludes  = []string{
		"metadata.*",
		"status.*",
	}
	extraExcludes = []string{}
	webhooks      = []string{}
)

func NewNotifyCommand() *cobra.Command {
	cmd := cobra.Command{
		Use: "notify",
	}

	cmd.PersistentFlags().StringSliceVar(&resources, "resources", resources, "The resource to watch")
	cmd.PersistentFlags().StringSliceVar(&excludes, "set-excludes", excludes, "The field should be exclude, clean and set")
	cmd.PersistentFlags().StringSliceVar(&extraExcludes, "add-excludes", extraExcludes, "The field should be exclude, add")
	cmd.PersistentFlags().StringSliceVar(&webhooks, "webhooks", webhooks, "The urls of webhook")

	cmd.RunE = func(*cobra.Command, []string) error {
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

		notifyFunc := service.StdoutNotify()
		if len(webhooks) > 0 {
			notifyFunc = service.WebhooksNotify(webhooks)
		}
		for _, resource := range resources {
			ctl, err := service.NewResourceController(
				resource, kubeClient, notifyFunc, rExcludes...)
			if err != nil {
				return err
			}
			stopCh := make(chan struct{})
			go ctl.Run(stopCh)
			defer close(stopCh)
		}

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM)
		signal.Notify(sigterm, syscall.SIGINT)
		<-sigterm

		return nil
	}

	return &cmd
}
