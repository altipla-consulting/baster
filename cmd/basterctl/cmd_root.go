package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var debugApp bool

func init() {
	CmdRoot.PersistentFlags().BoolVarP(&debugApp, "debug", "d", false, "Activa el logging de depuración")
}

var CmdRoot = &cobra.Command{
	Use:          "basterctl",
	Short:        "Controla y ayuda con la administración de una instancia baster en producción",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debugApp {
			log.SetLevel(log.DebugLevel)
			log.Debug("DEBUG log level activated")
		}
	},
}
