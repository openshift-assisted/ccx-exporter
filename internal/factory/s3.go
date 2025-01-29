package factory

import (
	"context"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"
	"github.com/go-logr/logr"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
)

func CreateS3Client(ctx context.Context, conf config.S3) (*s3.Client, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(conf.Creds.AccessKeyID, conf.Creds.SecretAccessKey, "")),
		awsconfig.WithRegion(conf.Region),
		awsconfig.WithLogger(AWSLogger{log.Logger()}),
	)

	if conf.BaseEndpoint != "" {
		baseEndpoint := conf.BaseEndpoint

		if !strings.HasPrefix(baseEndpoint, "http://") && !strings.HasPrefix(baseEndpoint, "https://") {
			baseEndpoint = fmt.Sprintf("https://%s", baseEndpoint)
		}

		awsConfig.BaseEndpoint = &baseEndpoint
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create aws config: %w", err)
	}

	ret := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = conf.UsePathStyle
	})

	return ret, nil
}

type AWSLogger struct {
	logger logr.Logger
}

func (a AWSLogger) Logf(classification logging.Classification, format string, v ...interface{}) {
	level := 0

	switch classification {
	case logging.Debug:
		level = 3
	case logging.Warn:
		level = 0
	default:
		return
	}

	msg := fmt.Sprintf(format, v...)

	a.logger.V(level).Info(msg)
}
