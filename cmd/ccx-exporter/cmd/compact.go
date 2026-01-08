package cmd

import (
	"context"
	"fmt"

	promversion "github.com/prometheus/common/version"
	"github.com/spf13/cobra"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/compact"
	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo/projectedevent"
	"github.com/openshift-assisted/ccx-exporter/internal/factory"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/internal/version"
)

var compactConfig *config.CompactConfig

// compactCmd represents the compact command
var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact s3 objects per date",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		compactConfig, err = config.ParseCompact(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to parse config %s: %w", cfgFile, err)
		}

		// Init logger
		err = log.Init(compactConfig.Logs)
		if err != nil {
			return fmt.Errorf("failed to init logger: %w", err)
		}

		logger := log.Logger()

		// Dump generic information
		logger.Info("Starting compact job",
			"revision", version.Revision,
			"branch", version.Branch,
			"buildContext", promversion.BuildContext(),
		)
		logger.Info("Using config", "config", fmt.Sprintf("%+v", compactConfig))

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.Logger()

		// Create main context
		// Listen to sigterm and interrupt signals
		rootCtx := context.Background()
		ctx := common.SetupSignalHandler(rootCtx)

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

		// Create S3 repo
		inputS3Client, err := factory.CreateS3Client(ctx, compactConfig.Input)
		if err != nil {
			logger.Error(err, "failed to create input s3 client")

			return
		}

		s3reader := projectedevent.NewS3Repo(inputS3Client, compactConfig.Input.Bucket, compactConfig.Input.KeyPrefix)

		outputS3Client, err := factory.CreateS3Client(ctx, compactConfig.Output)
		if err != nil {
			logger.Error(err, "failed to create output s3 client")

			return
		}

		s3writer := projectedevent.NewS3Repo(outputS3Client, compactConfig.Output.Bucket, compactConfig.Output.KeyPrefix)

		// Create task lister
		var taskLister compact.TaskLister
		switch {
		case compactConfig.Since != nil:
			taskLister = compact.NewSinceDateLister(*compactConfig.Since)
		default:
			logger.Info("using default task lister: only yesterday")

			taskLister = compact.OnlyYesterdayLister{}
		}

		// Create compact processor
		processor := compact.NewProcessor(s3reader, s3writer, taskLister, compactConfig.Parallel, compactConfig.MaxSize)

		err = processor.Compact(ctx)
		if err != nil {
			logger.Error(err, "failed to compact")

			return
		}

		logger.Info("Job done")
	},
}

func init() {
	rootCmd.AddCommand(compactCmd)
}
