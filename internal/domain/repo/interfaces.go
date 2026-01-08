package repo

import (
	"context"
	"time"

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

type ProjectedClusterEventWriter interface {
	WriteProjectedClusterEvent(ctx context.Context, event entity.ProjectedClusterEvent) error
}

type ProjectedClusterStateWriter interface {
	WriteProjectedClusterState(ctx context.Context, state entity.ProjectedClusterState) error
}

type ProjectedInfraEnvWriter interface {
	WriteProjectedInfraEnv(ctx context.Context, infraEnv entity.ProjectedInfraEnv) error
}

type ProjectionWriter interface {
	ProjectedClusterEventWriter
	ProjectedClusterStateWriter
	ProjectedInfraEnvWriter
}

type ProjectedClusterEventReader interface {
	ListProjectedClusterEvent(ctx context.Context, filter ListProjectionFilter) ([]entity.ProjectionMeta, error)
	GetProjectedClusterEvent(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedClusterEvent, error)
}

type ProjectedClusterStateReader interface {
	ListProjectedClusterState(ctx context.Context, filter ListProjectionFilter) ([]entity.ProjectionMeta, error)
	GetProjectedClusterState(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedClusterState, error)
}

type ProjectedInfraEnvReader interface {
	ListProjectedInfraEnv(ctx context.Context, filter ListProjectionFilter) ([]entity.ProjectionMeta, error)
	GetProjectedInfraEnv(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedInfraEnv, error)
}

type ProjectionReader interface {
	ProjectedClusterEventReader
	ProjectedClusterStateReader
	ProjectedInfraEnvReader
}

type ListProjectionFilter struct {
	Date time.Time
}
