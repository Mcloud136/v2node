package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use: "v2node",
}

func Run() {
	err := command.Execute()
	if err != nil {
		log.WithField("err", err).Error("Execute command failed")
		os.Exit(1)
	}
}
