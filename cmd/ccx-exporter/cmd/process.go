package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	promversion "github.com/prometheus/common/version"
	"github.com/spf13/cobra"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo/processingerror"
	"github.com/openshift-assisted/ccx-exporter/internal/factory"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/internal/processing"
	"github.com/openshift-assisted/ccx-exporter/internal/version"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

var conf *config.Config

// processCmd represents the process command
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process kafka events and push it to s3",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		conf, err = config.Parse(cfgFile)
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
			"revision", version.Revision,
			"branch", version.Branch,
			"buildContext", promversion.BuildContext(),
		)
		logger.Info("Using config", "config", fmt.Sprintf("%+v", conf))

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

		// Create & start prometheus server
		registry := prometheus.NewRegistry()
		promserver := factory.CreatePrometheusServer(conf.Metrics, registry)

		go func() {
			err := promserver.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logger.Error(err, "Prometheus server stopped")
			}
		}()

		defer func() {
			ctx, cancel := context.WithTimeout(rootCtx, conf.GracefulDuration)
			defer cancel()

			err := promserver.Shutdown(ctx)
			if err != nil {
				logger.Error(err, "failed to close prometheus server")
			}
		}()

		// Create Kafka client
		kc, err := factory.CreateKafkaConsumer(conf.Kafka)
		if err != nil {
			logger.Error(err, "failed to create kafka consumer group")

			return
		}

		defer func() {
			err := kc.Close()
			if err != nil {
				logger.Error(err, "failed to close kafka consumer")
			}
		}()

		// Create S3 clients
		s3Client, err := factory.CreateS3Client(ctx, conf.S3)
		if err != nil {
			logger.Error(err, "failed to create main s3 client")

			return
		}

		dlqS3Client, err := factory.CreateS3Client(ctx, conf.DeadLetterQueue)
		if err != nil {
			logger.Error(err, "failed to create dlq s3 client")

			return
		}

		// Create Valkey client
		valkeyClient, err := factory.CreateValkeyClient(ctx, conf.Valkey)
		if err != nil {
			logger.Error(err, "failed to create valkey client")

			return
		}

		defer func() {
			valkeyClient.Close()
		}()

		// Create S3 repo for processing error
		processingErrorWriter := processingerror.NewS3Writer(dlqS3Client, conf.DeadLetterQueue.Bucket, conf.DeadLetterQueue.KeyPrefix)

		// Create Main Processing
		mainProcessing := processing.NewMain(s3Client, valkeyClient)

		decoratedProcessing, err := factory.DecorateProcessing(mainProcessing, registry)
		if err != nil {
			logger.Error(err, "failed to create decorated processing")

			return
		}

		// Create Error Processing
		errorProcessing := processing.NewMainError(processingErrorWriter)

		decoratedErrorProcessing, err := factory.DecorateErrorProcessing(errorProcessing, registry)
		if err != nil {
			logger.Error(err, "failed to create decorated error processing")

			return
		}

		// Create Runner & Start processing
		topics := strings.Split(conf.Kafka.Consumer.Topic, ",")

		runner := pipeline.NewRunner(kc, topics, decoratedProcessing, decoratedErrorProcessing).WithLogger(logger)

		logger.V(2).Info("Start Processing")

		err = runner.Run(ctx)
		if err != nil {
			logger.Error(err, "runner stopped unexpectedly")
		}

		logger.V(2).Info("Processing stopped")
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
