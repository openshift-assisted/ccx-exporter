package processing

import (
	"context"

	"github.com/IBM/sarama"
)

// No interface
type Pipeline interface {
	Start(ctx context.Context) error
}

type Worker interface {
	Process(ctx context.Context /* custom type */, msg sarama.ConsumerMessage) error
}

type ErrorWorker interface {
	Process(ctx context.Context, err ProcessingError, msg sarama.ConsumerMessage) error
}

type ProcessingError interface {
	error
	Type() string
	AdditionalInputs() []interface{}
}
