package pipeline

import (
	"errors"
	"fmt"
)

// ErrProcessingError

type ErrProcessingError struct {
	error
	Category         string
	AdditionalInputs []Input
}

type Input struct {
	Key   string
	Value []byte
}

const (
	UnknownCategory        = "unknown"
	UnmarshalErrorCategory = "unmarshal"
	PanicCategory          = "panic"
)

func NewErrProcessingError(err error, category string, additionalInputs []Input) ErrProcessingError {
	return ErrProcessingError{
		error:            err,
		Category:         category,
		AdditionalInputs: additionalInputs,
	}
}

func (e ErrProcessingError) Unwrap() error {
	return e.error
}

// ErrRetryableError

var ErrRetryableError = errors.New("retryable error")

func NewErrRetryableError(err error) error {
	return fmt.Errorf("%w: %w", ErrRetryableError, err)
}

func NewRetryableErrProcessingError(err error, category string, additionalInputs []Input) ErrProcessingError {
	return ErrProcessingError{
		error:            NewErrRetryableError(err),
		Category:         category,
		AdditionalInputs: additionalInputs,
	}
}
