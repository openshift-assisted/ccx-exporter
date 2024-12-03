package cmd

import (
	"context"
	"fmt"

	"github.com/prometheus/common/version"
	"github.com/spf13/cobra"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
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
		logger.Info("Using config", "config", fmt.Sprintf("%+v", conf))

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.Logger()

		// Set max procs based on cpu limits
		err := common.SetMaxProcs()
		if err != nil {
			logger.Error(err, "failed to set max procs")

			return
		}

		// Set max memory
		err = common.SetMemLimit()
		if err != nil {
			logger.Error(err, "failed to set mem limit")

			return
		}

		// Listen to sigterm and interrupt signals
		ctx := common.SetupSignalHandler(context.Background())

		// Create pipeline

		// Start pipeline

		logger.V(2).Info("Processing stopped")
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
