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

type HostStateWriter interface {
	WriteHostState(ctx context.Context, state entity.HostState) error
}

type HostStateReader interface {
	GetHostStates(ctx context.Context, clusterID string) ([]entity.HostState, error)
}

type HostState interface {
	HostStateWriter
	HostStateReader
}

type ProjectedEventWriter interface {
	WriteProjectedEvent(ctx context.Context, event entity.ProjectedEvent) error
}

type ProjectedEvent interface {
	ProjectedEventWriter
}
