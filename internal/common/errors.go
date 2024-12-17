package common

import (
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

func NewErrProcessingError(err error, category string, inputs []pipeline.Input, reason string, args ...interface{}) pipeline.ErrProcessingError {
	cause := fmt.Sprintf(reason, args...)
	dErr := fmt.Errorf("%s: %w", cause, err)

	return pipeline.NewErrProcessingError(dErr, category, inputs)
}

func NewRetryableErrProcessingError(err error, category string, inputs []pipeline.Input, reason string, args ...interface{}) pipeline.ErrProcessingError {
	return NewErrProcessingError(pipeline.NewErrRetryableError(err), category, inputs, reason, args...)
}
