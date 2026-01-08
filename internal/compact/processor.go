package compact

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
)

// Lister

type TaskLister interface {
	ListTasks(ctx context.Context) ([]Task, error)
}

type Task struct {
	Date time.Time
	Type entity.Type
}

func (t Task) DateStr() string {
	return fmt.Sprintf("%04d-%02d-%02d", t.Date.Year(), t.Date.Month(), t.Date.Day())
}

// Processor

type Processor struct {
	parallel int
	maxSize  int
	pool     *sync.Pool

	reader repo.ProjectionReader
	writer repo.ProjectionWriter

	lister TaskLister
}

func NewProcessor(reader repo.ProjectionReader, writer repo.ProjectionWriter, lister TaskLister, parallel int, maxSize int) Processor {
	return Processor{
		reader: reader,
		writer: writer,
		lister: lister,

		parallel: parallel,
		maxSize:  maxSize,
		pool: &sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, maxSize))
			},
		},
	}
}

func (p Processor) Compact(ctx context.Context) error {
	logger := log.Logger()

	logger.Info("Start processing")

	// List the date to process
	tasks, err := p.lister.ListTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list task: %w", err)
	}

	logger.V(4).Info("list of task", "tasks", tasks)

	// Start N runners to process the different tasks in parallel
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(p.parallel)

	for _, task := range tasks {
		g.Go(p.processTask(ctx, task))
	}

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	logger.Info("Processing is done")

	return nil
}

func (p Processor) processTask(ctx context.Context, task Task) func() error {
	return func() error {
		logger := log.Logger()

		logger.V(4).Info("Start processing task", "type", string(task.Type), "date", task.DateStr())

		start := time.Now()
		defer func() {
			logger.V(4).Info("Processing task is done", "type", string(task.Type), "date", task.DateStr(), "elapsed", time.Since(start).String())
		}()

		// List projections
		metas, err := p.listProjection(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to list projection: %w", err)
		}

		if len(metas) == 0 {
			return nil
		}

		// Read in parallel all payload to copy
		payloads := make(chan []byte, 10)

		g, readCtx := errgroup.WithContext(ctx)
		g.SetLimit(10)

		go func() {
			for _, m := range metas {
				meta := m

				g.Go(func() error {
					projection, err := p.getProjection(readCtx, task, meta)
					if err != nil {
						return fmt.Errorf("failed to get projection (%v) (%v): %w", task, meta, err)
					}

					select {
					case payloads <- projection.Payload:
						return nil
					case <-readCtx.Done():
						return readCtx.Err()
					}
				})
			}

			g.Wait() //nolint // The value is checked at the end, to be sure the error can be propagated.
			close(payloads)
		}()

		// Loop and create up to maxSize byte compacted objects
		creationIndex := 0

		buf := p.pool.Get().(*bytes.Buffer)
		buf.Reset()

		defer func() {
			buf.Reset()
			p.pool.Put(buf)
		}()

		for payload := range payloads {
			// append to buf
			_, err = buf.Write(payload)
			if err != nil {
				return fmt.Errorf("failed to add projection payload to buffer")
			}

			err = buf.WriteByte('\n')
			if err != nil {
				return fmt.Errorf("failed to add new line to buffer")
			}

			// push chunk
			if buf.Len() > p.maxSize {
				err = p.writeProjection(ctx, task, buf, &creationIndex)
				if err != nil {
					return fmt.Errorf("failed to write projection: %w", err)
				}
			}
		}

		// last chunk
		if buf.Len() > 0 {
			err = p.writeProjection(ctx, task, buf, &creationIndex)
			if err != nil {
				return fmt.Errorf("failed to write projection: %w", err)
			}
		}

		// check all read were successful
		err = g.Wait()
		if err != nil {
			return fmt.Errorf("failed to read all payload: %w", err)
		}

		return nil
	}
}

func (p Processor) listProjection(ctx context.Context, task Task) ([]entity.ProjectionMeta, error) {
	switch task.Type {
	case entity.ClusterEventType:
		return p.reader.ListProjectedClusterEvent(ctx, repo.ListProjectionFilter{Date: task.Date})
	case entity.ClusterStateType:
		return p.reader.ListProjectedClusterState(ctx, repo.ListProjectionFilter{Date: task.Date})
	case entity.InfraEnvType:
		return p.reader.ListProjectedInfraEnv(ctx, repo.ListProjectionFilter{Date: task.Date})
	default:
		return nil, fmt.Errorf("unexpected type: %v", task.Type)
	}
}

func (p Processor) getProjection(ctx context.Context, task Task, meta entity.ProjectionMeta) (entity.Projection, error) {
	switch task.Type {
	case entity.ClusterEventType:
		ret, err := p.reader.GetProjectedClusterEvent(ctx, meta)

		return entity.Projection(ret), err
	case entity.ClusterStateType:
		ret, err := p.reader.GetProjectedClusterState(ctx, meta)

		return entity.Projection(ret), err
	case entity.InfraEnvType:
		ret, err := p.reader.GetProjectedInfraEnv(ctx, meta)

		return entity.Projection(ret), err
	default:
		return entity.Projection{}, fmt.Errorf("unexpected type: %v", task.Type)
	}
}

func (p Processor) writeProjection(ctx context.Context, task Task, buf *bytes.Buffer, index *int) error {
	projection := entity.Projection{
		Meta: entity.ProjectionMeta{
			ID:        fmt.Sprintf("data%d", *index),
			Timestamp: task.Date,
		},
		Payload: buf.Bytes(),
	}

	defer func() {
		*index++
		buf.Reset()
	}()

	switch task.Type {
	case entity.ClusterEventType:
		return p.writer.WriteProjectedClusterEvent(ctx, entity.ProjectedClusterEvent(projection))
	case entity.ClusterStateType:
		return p.writer.WriteProjectedClusterState(ctx, entity.ProjectedClusterState(projection))
	case entity.InfraEnvType:
		return p.writer.WriteProjectedInfraEnv(ctx, entity.ProjectedInfraEnv(projection))
	default:
		return fmt.Errorf("unexpected type: %v", task.Type)
	}
}
