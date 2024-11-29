package cmd

import (
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/prometheus/common/version"
	"github.com/spf13/cobra"
)

// processCmd represents the process command
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process kafka events and push it to s3",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		conf, err := config.Parse(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to parse config %s: %w", cfgFile, err)
		}

		// Init logger
		err = log.Init(conf.Logs)
		if err != nil {
			return fmt.Errorf("failed to init logger: %w", err)
		}

		logger := log.Logger()

		// Dump generic information
		logger.Info("Starting ccx exporter",
			"version", version.Info(),
			"buildContext", version.BuildContext(),
		)
		logger.Info("Using config", "config", conf)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("process called")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
