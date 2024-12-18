package pipeline

import (
	"errors"
	"fmt"

	"github.com/IBM/sarama"
)

// ErrProcessingError

type ErrProcessingError struct {
	error
	Category         string
	Event            *sarama.ConsumerMessage
	AdditionalInputs []Input
}

type Input struct {
	Source string
	Key    string
	Value  []byte
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
	return NewErrProcessingError(NewErrRetryableError(err), category, additionalInputs)
}
