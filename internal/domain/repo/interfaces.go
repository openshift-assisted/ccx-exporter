package repo

import (
	"context"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

//go:generate mockgen -source=interfaces.go -package=mock -destination=./mock/mock_repo.go

type ProcessingErrorWriter interface {
	WriteProcessingError(ctx context.Context, pErr pipeline.ErrProcessingError) error
}

type ProcessingError interface {
	ProcessingErrorWriter
}

type EventWriter interface {
	WriteEvent(ctx context.Context, event entity.Event) error
}

type EventReader interface {
	GetEvent(ctx context.Context, clusterID string) (entity.Event, error)
}

type Event interface {
	EventWriter
	EventReader
}

type ProjectedEventWriter interface {
	WriteProjectedEvent(ctx context.Context, event entity.ProjectedEvent) error
}

type ProjectedEvent interface {
	ProjectedEventWriter
}
