package pipeline

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/go-logr/logr"
)

type Runner[Payload any] struct {
	consumer sarama.ConsumerGroup
	topics   []string

	handler JSONHandler[Payload]

	logger *logr.Logger
}

func NewRunner[Payload any](consumer sarama.ConsumerGroup, topics []string, processing Processing[Payload], errorProcessing ErrorProcessing) Runner[Payload] {
	handler := NewJSONHandler(processing, errorProcessing)

	return Runner[Payload]{
		consumer: consumer,
		topics:   topics,
		handler:  handler,
	}
}

func (r Runner[Payload]) WithLogger(logger logr.Logger) Runner[Payload] {
	r.logger = &logger
	r.handler = r.handler.WithLogger(logger)

	return r
}

func (r Runner[Payload]) Start(ctx context.Context) error {
	go func() {
		for err := range r.consumer.Errors() {
			r.logError(err, "kafka consumer error")
		}
	}()

	for {
		err := r.consumer.Consume(ctx, r.topics, r.handler)
		if err != nil {
			r.logError(err, "Consumer failed")

			return fmt.Errorf("consumer failed: %w", err)
		}

		// If context is cancelled, no need to keep looping
		err = ctx.Err()
		if err != nil {
			r.logInfo(0, "Context expired")

			return err
		}
	}
}

func (r Runner[Payload]) logInfo(level int, msg string, keysAndValues ...any) {
	if r.logger == nil {
		return
	}

	r.logger.V(level).Info(msg, keysAndValues...)
}

func (r Runner[Payload]) logError(err error, msg string, keysAndValues ...any) {
	if r.logger == nil {
		return
	}

	r.logger.Error(err, msg, keysAndValues...)
}
