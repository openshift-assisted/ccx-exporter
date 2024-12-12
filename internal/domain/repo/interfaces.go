package repo

import (
	"context"

	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

type ProcessingErrorWriter interface {
	WriteProcessingError(ctx context.Context, pErr pipeline.ErrProcessingError) error
}

type ProcessingError interface {
	ProcessingErrorWriter
}
