package processing

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

type MainError struct {
	s3Client *s3.Client
}

func NewMainError(s3Client *s3.Client) MainError {
	return MainError{
		s3Client: s3Client,
	}
}

func (m MainError) Process(ctx context.Context, pErr pipeline.ErrProcessingError) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}
