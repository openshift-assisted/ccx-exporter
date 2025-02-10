package processing

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

type CountData struct {
	counter *prometheus.CounterVec
	inner   pipeline.Processing[entity.Event]
}

func NewCountData(p pipeline.Processing[entity.Event], registry prometheus.Registerer, config pipeline.MetricsConfig) (pipeline.Processing[entity.Event], error) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Name:      "data_total",
		Help:      "Data counter by event name.",
	}, []string{"name"})

	err := registry.Register(counter)
	if err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	ret := CountData{
		counter: counter,
		inner:   p,
	}

	return ret, nil
}

func (p CountData) Process(ctx context.Context, event entity.Event) error {
	defer p.counter.WithLabelValues(event.Name).Inc()

	return p.inner.Process(ctx, event)
}
