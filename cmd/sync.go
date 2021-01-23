package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/j2gg0s/kubenotify/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	service "github.com/j2gg0s/kubenotify/pkg/service/sync"
)

func NewSyncCommand() *cobra.Command {
	cmd := cobra.Command{
		Use: "sync",
	}

	var (
		namespace string
		resources []string
		excludes  []string
		cluster   string
		addrs     []string
	)

	cmd.PersistentFlags().StringVar(&namespace, "namespace", "", "concerned namespace, default all namespace")
	cmd.PersistentFlags().StringSliceVar(&resources, "resource", []string{}, "concerned resources, example pods, deployments")
	cmd.PersistentFlags().StringSliceVar(&excludes, "exclude", []string{}, "exlcude namesapce/resource, support regex, example: default/demo")
	cmd.PersistentFlags().StringVar(&cluster, "cluster", "", "name of current cluster")
	cmd.PersistentFlags().StringSliceVar(&addrs, "addrs", []string{}, "kafka's addrs")

	cmd.RunE = func(*cobra.Command, []string) error {
		producer, err := client.NewKafkaProducer(addrs)
		if err != nil {
			return err
		}

		for _, resource := range resources {
			ctl, err := service.NewController(
				namespace, resource, kubeClient,
				func(e service.Event) error {
					return service.SendMessage(context.Background(), producer, cluster, e)
				},
				log.Logger)
			if err != nil {
				return err
			}
			stopC := make(chan struct{})
			defer close(stopC)
			go ctl.Run(1, stopC)
		}

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM)
		signal.Notify(sigterm, syscall.SIGINT)
		<-sigterm
		return nil
	}

	return &cmd
}
