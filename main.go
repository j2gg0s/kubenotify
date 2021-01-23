package main

import (
	"github.com/rs/zerolog/log"

	"github.com/j2gg0s/kubenotify/cmd"
)

func main() {
	root := cmd.NewRootCommand()

	root.AddCommand(
		cmd.NewNotifyCommand(),
		cmd.NewSyncCommand(),
	)

	if err := root.Execute(); err != nil {
		log.Err(err).Send()
	}
}
