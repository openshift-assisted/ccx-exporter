package processing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

var errNotImplemented = errors.New("not implemented")

const (
	eventNameEvent         = "Event"
	eventNameClusterState  = "ClusterState"
	eventNameHostState     = "HostState"
	eventNameInfraEnvState = "InfraEnv"

	categoryUnknownEventName = "unknown_name"
)

type Main struct {
	hostRepo         repo.HostState
	projectionWriter repo.ProjectionWriter
}

func NewMain(hostRepo repo.HostState, projectionWriter repo.ProjectionWriter) Main {
	return Main{
		hostRepo:         hostRepo,
		projectionWriter: projectionWriter,
	}
}

func (m Main) Process(processingCtx context.Context, event entity.Event) error {
	ctx, cancel := context.WithTimeout(processingCtx, 4*time.Second)
	defer cancel()

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
		return pipeline.NewErrProcessingError(fmt.Errorf("unknown event name: %s", event.Name), categoryUnknownEventName, nil)
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
