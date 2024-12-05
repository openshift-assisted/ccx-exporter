package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

// Parallel Processing

type parallel[Payload any] struct {
	procs []Processing[Payload]
}

func NewParallelProcessing[Payload any](p ...Processing[Payload]) Processing[Payload] {
	return parallel[Payload]{
		procs: p,
	}
}

func (p parallel[Payload]) Process(ctx context.Context, payload Payload) error {
	group, ctx := errgroup.WithContext(ctx)

	for _, proc := range p.procs {
		processing := proc

		group.Go(func() error {
			return processing.Process(ctx, payload)
		})
	}

	return group.Wait()
}

// Panic handler Processing

type panicHandler[Payload any] struct {
	processing Processing[Payload]
}

func NewPanicHandlerProcessing[Payload any](p Processing[Payload]) Processing[Payload] {
	return panicHandler[Payload]{
		processing: p,
	}
}

func (p panicHandler[Payload]) Process(ctx context.Context, payload Payload) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = NewErrProcessingError(
				fmt.Errorf("unexpected error: %v", r),
				PanicCategory,
				nil,
			)
		}
	}()

	err = p.processing.Process(ctx, payload)

	return
}

// Retry Processing

type retryProcessing[Payload any] struct {
	processing Processing[Payload]
	config     RetryConfig
}

type RetryConfig struct {
	MaxAttempt uint
	Delay      time.Duration
}

func NewRetryProcessing[Payload any](p Processing[Payload], config RetryConfig) Processing[Payload] {
	return retryProcessing[Payload]{
		processing: p,
		config:     config,
	}
}

func (p retryProcessing[Payload]) Process(ctx context.Context, payload Payload) error {
	return retry.Do(
		func() error {
			return p.processing.Process(ctx, payload)
		},
		retry.Context(ctx),
		retry.Attempts(p.config.MaxAttempt),
		retry.RetryIf(func(err error) bool {
			return errors.Is(err, ErrRetryableError)
		}),
		retry.Delay(p.config.Delay),
		retry.LastErrorOnly(true),
	)
}

// Duration Metric Processing

type MetricsConfig struct {
	Namespace string
	Buckets   []float64
}

type durationDecorator[Payload any] struct {
	processing Processing[Payload]
	histogram  *prometheus.HistogramVec
	clock      clockwork.Clock
}

func NewDurationMetricsDecoratorProcessing[Payload any](p Processing[Payload], registry prometheus.Registerer, clock clockwork.Clock, config MetricsConfig) (Processing[Payload], error) {
	ret := durationDecorator[Payload]{
		processing: p,
		clock:      clock,
	}

	buckets := config.Buckets
	if len(buckets) == 0 {
		buckets = []float64{10, 20, 50, 100, 200, 500, 1000, 2000, 5000}
	}

	opts := prometheus.HistogramOpts{
		Namespace: config.Namespace,
		Name:      "processing_duration_milliseconds",
		Help:      "Time taken to process payload.",
		Buckets:   buckets,
	}

	histogram := prometheus.NewHistogramVec(opts, []string{"failed"})

	err := registry.Register(histogram)
	if err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	ret.histogram = histogram

	return ret, nil
}

func (p durationDecorator[Payload]) Process(ctx context.Context, payload Payload) error {
	start := p.clock.Now()

	err := p.processing.Process(ctx, payload)

	duration := p.clock.Since(start)
	durationMilli := float64(duration/time.Millisecond) + float64(duration%time.Millisecond)/float64(time.Millisecond)

	p.histogram.WithLabelValues(fmt.Sprintf("%v", err != nil)).Observe(durationMilli)

	return err
}

// Error Metric Processing

type errorCountProcessing struct {
	counter *prometheus.CounterVec
}

func NewErrorCountProcessing(registry prometheus.Registerer, config MetricsConfig) (Processing[ErrProcessingError], error) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Name:      "processing_error_total",
		Help:      "Error counter by category.",
	}, []string{"category"})

	err := registry.Register(counter)
	if err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	ret := errorCountProcessing{
		counter: counter,
	}

	return ret, nil
}

func (p errorCountProcessing) Process(ctx context.Context, processingError ErrProcessingError) error {
	category := processingError.Category
	if category == "" {
		category = "empty-category"
	}

	p.counter.WithLabelValues(category).Inc()

	return nil
}
