package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/IBM/sarama"
	"github.com/go-logr/logr"
)

type JSONHandler[Payload any] struct {
	logger *logr.Logger

	processing      Processing[Payload]
	errorProcessing ErrorProcessing
}

func NewJSONHandler[Payload any](processing Processing[Payload], errProcessing ErrorProcessing) JSONHandler[Payload] {
	return JSONHandler[Payload]{
		processing:      processing,
		errorProcessing: errProcessing,
	}
}

func (h JSONHandler[Payload]) WithLogger(logger logr.Logger) JSONHandler[Payload] {
	h.logger = &logger

	return h
}

func (h JSONHandler[Payload]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()

	h.logInfo(0, "Start consuming",
		"topic", claim.Topic(),
		"partition", claim.Partition(),
		"initialOffset", claim.InitialOffset(),
	)

	for msg := range claim.Messages() {
		// If a re-balancing occurred, context will be canceled
		// Could also be a termination signal or anything
		if ctx.Err() != nil {
			break
		}

		if msg == nil {
			h.logInfo(1, "Nil message")

			continue
		}

		h.logInfo(3, "Processing message", "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)

		payload := new(Payload)

		err := json.Unmarshal(msg.Value, payload)
		if err != nil { // Not retryable
			h.processError(ctx, msg, NewErrProcessingError(err, UnmarshalErrorCategory, nil), session)

			continue
		}

		err = h.processing.Process(ctx, *payload)
		if err != nil {
			h.processError(ctx, msg, err, session)

			continue
		}

		session.MarkMessage(msg, "")
	}

	return nil
}

func (h JSONHandler[Payload]) processError(ctx context.Context, msg *sarama.ConsumerMessage, pipelineError error, session sarama.ConsumerGroupSession) {
	// If context has been cancelled, don't commit offset. Message will be reprocessed with a valid context
	err := ctx.Err()
	if err != nil {
		h.logInfo(1, "Not processing error, context has been cancelled")

		return
	}

	defer session.MarkMessage(msg, "")

	h.logError(pipelineError, "Processing failed")

	processingError := createProcessingError(pipelineError)

	err = h.errorProcessing.Process(ctx, processingError)
	if err != nil {
		h.logError(err, "Error pipeline failed")

		h.dumpErrorContext(msg, processingError)
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (h JSONHandler[Payload]) Setup(session sarama.ConsumerGroupSession) error {
	h.logInfo(0, "Setup to consume", "claims", session.Claims())

	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
// but before the offsets are committed for the very last time.
func (h JSONHandler[Payload]) Cleanup(session sarama.ConsumerGroupSession) error {
	h.logInfo(0, "Cleanup after consuming", "claims", session.Claims())

	return nil
}

func (h JSONHandler[Payload]) dumpErrorContext(msg *sarama.ConsumerMessage, err ErrProcessingError) {
	h.logger.Error(err,
		"Failed to process message",
		"kafka.topic", msg.Topic,
		"kafka.partition", msg.Partition,
		"kafka.offset", msg.Offset,
		"kafka.payload", msg.Value,
		"additionalInputs", err.AdditionalInputs,
		"category", err.Category,
	)
}

func (h JSONHandler[Payload]) logInfo(level int, msg string, keysAndValues ...any) {
	if h.logger == nil {
		return
	}

	h.logger.V(level).Info(msg, keysAndValues...)
}

func (h JSONHandler[Payload]) logError(err error, msg string, keysAndValues ...any) {
	if h.logger == nil {
		return
	}

	h.logger.Error(err, msg, keysAndValues...)
}

func createProcessingError(err error) ErrProcessingError {
	ret := ErrProcessingError{}
	if errors.As(err, &ret) {
		return ret
	}

	return NewErrProcessingError(err, UnknownCategory, nil)
}
