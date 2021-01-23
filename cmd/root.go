package cmd

import (
	"github.com/j2gg0s/kubenotify/pkg/client"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

var (
	debug        = false
	outofCluster = false
	kubeClient   kubernetes.Interface
)

func NewRootCommand() *cobra.Command {
	cmd := cobra.Command{
		Use: "kubenotify",
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", debug, "enable debug log")
	cmd.PersistentFlags().BoolVar(&outofCluster, "outof-cluster", outofCluster, "use outof cluster config directly")

	cmd.PersistentPreRunE = func(*cobra.Command, []string) error {
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

	return &cmd
}
