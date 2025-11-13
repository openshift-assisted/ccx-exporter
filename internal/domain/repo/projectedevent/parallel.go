package projectedevent

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
)

type ParallelWriter struct {
	writers []repo.ProjectionWriter
}

func NewParallelWriter(writers ...repo.ProjectionWriter) ParallelWriter {
	return ParallelWriter{
		writers: writers,
	}
}

func (p ParallelWriter) WriteProjectedClusterEvent(ctx context.Context, event entity.ProjectedClusterEvent) error {
	group, ctx := errgroup.WithContext(ctx)

	for _, w := range p.writers {
		writer := w

		group.Go(func() error {
			return writer.WriteProjectedClusterEvent(ctx, event)
		})
	}

	return group.Wait()
}

func (p ParallelWriter) WriteProjectedClusterState(ctx context.Context, state entity.ProjectedClusterState) error {
	group, ctx := errgroup.WithContext(ctx)

	for _, w := range p.writers {
		writer := w

		group.Go(func() error {
			return writer.WriteProjectedClusterState(ctx, state)
		})
	}

	return group.Wait()
}

func (p ParallelWriter) WriteProjectedInfraEnv(ctx context.Context, infraEnv entity.ProjectedInfraEnv) error {
	group, ctx := errgroup.WithContext(ctx)

	for _, w := range p.writers {
		writer := w

		group.Go(func() error {
			return writer.WriteProjectedInfraEnv(ctx, infraEnv)
		})
	}

	return group.Wait()
}
