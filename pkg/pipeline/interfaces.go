package pipeline

import "context"

//go:generate mockgen -source=interfaces.go -package=mock -destination=./mock/mock_pipeline.go

type Processing[Payload any] interface {
	Process(context.Context, Payload) error
}

type ErrorProcessing Processing[ErrProcessingError]
