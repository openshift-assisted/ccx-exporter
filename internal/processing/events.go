package processing

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

var errNotImplemented = errors.New("not implemented")

type Main struct {
	s3Client *s3.Client
}

func NewMain(s3Client *s3.Client) Main {
	return Main{
		s3Client: s3Client,
	}
}

func (m Main) Process(ctx context.Context, event entity.Event) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}
