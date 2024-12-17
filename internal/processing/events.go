package processing

import (
	"context"
	"errors"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

var (
	errUnknownEvent   = errors.New("unknown event name")
	errNotImplemented = errors.New("not implemented")
)

const (
	eventNameEvent         = "Event"
	eventNameClusterState  = "ClusterState"
	eventNameHostState     = "HostState"
	eventNameInfraEnvState = "InfraEnv"
)

type Main struct {
	hostRepo        repo.HostState
	projectedWriter repo.ProjectedEventWriter
}

func NewMain(hostRepo repo.HostState, projectedWriter repo.ProjectedEventWriter) Main {
	return Main{
		hostRepo:        hostRepo,
		projectedWriter: projectedWriter,
	}
}

func (m Main) Process(ctx context.Context, event entity.Event) error {
	switch event.Name {
	case eventNameEvent:
		return m.processClusterEvent(ctx, event)
	case eventNameClusterState:
		return m.processClusterState(ctx, event)
	case eventNameHostState:
		return m.processHostState(ctx, event)
	case eventNameInfraEnvState:
		return m.processInfraEnv(ctx, event)
	default:
		return pipeline.NewErrProcessingError(errUnknownEvent, "unknown_name", nil)
	}
}

func (m Main) processClusterEvent(ctx context.Context, event entity.Event) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}

func (m Main) processClusterState(ctx context.Context, event entity.Event) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}

func (m Main) processHostState(ctx context.Context, event entity.Event) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}

func (m Main) processInfraEnv(ctx context.Context, event entity.Event) error {
	return pipeline.NewErrProcessingError(errNotImplemented, "not_implemented", nil)
}
