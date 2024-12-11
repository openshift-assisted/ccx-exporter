package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/factory"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/internal/processing"
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
			"version", version.Info(),
			"buildContext", version.BuildContext(),
		)
		logger.Info("Using config", "config", fmt.Sprintf("%+v", conf))

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.Logger()

		// Create main context
		// Listen to sigterm and interrupt signals
		ctx := common.SetupSignalHandler(context.Background())

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

		// Start prometheus server
		registry := prometheus.NewRegistry()

		stopMetricServer := common.StartPrometheusServer(conf.Metrics, registry)

		// Create Kafka client
		kc, stopKafkaConsumer, err := factory.CreateKafkaConsumer(conf.Kafka)
		if err != nil {
			logger.Error(err, "failed to create kafka consumer group")

			return
		}

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
		valkeyClient, stopValkeyClient, err := factory.CreateValkeyClient(ctx, conf.Valkey)
		if err != nil {
			logger.Error(err, "failed to create valkey client")

			return
		}

		// Create Main Processing
		mainProcessing := processing.NewMain(s3Client, valkeyClient)

		decoratedProcessing, err := factory.DecorateProcessing(mainProcessing, registry)
		if err != nil {
			logger.Error(err, "failed to create decorated processing")

			return
		}

		// Create Error Processing
		errorProcessing := processing.NewMainError(dlqS3Client)

		decoratedErrorProcessing, err := factory.DecorateErrorProcessing(errorProcessing, registry)
		if err != nil {
			logger.Error(err, "failed to create decorated error processing")

			return
		}

		// GracefulShutdown
		go func() {
			<-ctx.Done()

			logger.V(2).Info("Starting shutdown")

			stopContext, cancel := context.WithTimeout(context.Background(), conf.GracefulDuration)
			defer cancel()

			group, _ := errgroup.WithContext(stopContext)

			group.Go(func() error {
				err := stopKafkaConsumer(stopContext)
				if err != nil {
					return fmt.Errorf("failed to gracefully close kafka consumer: %w", err)
				}

				return nil
			})

			group.Go(func() error {
				err := stopMetricServer(stopContext)
				if err != nil {
					return fmt.Errorf("failed to gracefully close metrics server: %w", err)
				}

				return nil
			})

			group.Go(func() error {
				err := stopValkeyClient(stopContext)
				if err != nil {
					return fmt.Errorf("failed to gracefully close valkey client: %w", err)
				}

				return nil
			})

			err := group.Wait()
			if err != nil {
				logger.Error(err, "Graceful shutdown failed")
			}
		}()

		// Create Runner & Start processing
		topics := strings.Split(conf.Kafka.Consumer.Topic, ",")

		runner := pipeline.NewRunner(kc, topics, decoratedProcessing, decoratedErrorProcessing).WithLogger(logger)
		logger.V(2).Info("Start Processing")

		err = runner.Start(ctx)
		if err != nil {
			logger.Error(err, "runner stopped unexpectedly")
		}

		logger.V(2).Info("Processing stopped")
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
