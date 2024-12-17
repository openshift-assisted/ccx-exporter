package projectedevent

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

type S3Writer struct {
	s3client *s3.Client

	bucket string
	prefix string
}

func NewS3Writer(s3client *s3.Client, bucket string, prefix string) S3Writer {
	return S3Writer{
		s3client: s3client,
		bucket:   bucket,
		prefix:   prefix,
	}
}

func (s S3Writer) WriteProjectedClusterEvent(ctx context.Context, event entity.ProjectedClusterEvent) error {
	return errors.New("not implemented")
}

func (s S3Writer) WriteProjectedClusterState(ctx context.Context, state entity.ProjectedClusterState) error {
	return errors.New("not implemented")
}

func (s S3Writer) WriteProjectedInfraEnv(ctx context.Context, infraEnv entity.ProjectedInfraEnv) error {
	return errors.New("not implemented")
}
