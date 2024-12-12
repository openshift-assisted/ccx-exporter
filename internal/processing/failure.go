package processing

import (
	"context"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

type MainError struct {
	pErrRepo repo.ProcessingErrorWriter
}

func NewMainError(pErrRepo repo.ProcessingErrorWriter) MainError {
	return MainError{
		pErrRepo: pErrRepo,
	}
}

func (m MainError) Process(ctx context.Context, pErr pipeline.ErrProcessingError) error {
	err := m.pErrRepo.WriteProcessingError(ctx, pErr)
	if err != nil {
		switch {
		default:
			return pipeline.NewErrProcessingError(err, "generic", nil)
		}
	}

	return nil
}
