package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

type CountLateData struct {
	counter *prometheus.CounterVec
	clock   clockwork.Clock
	inner   pipeline.Processing[entity.Event]
}

func NewCountLateData(p pipeline.Processing[entity.Event], registry prometheus.Registerer, clock clockwork.Clock, config pipeline.MetricsConfig) (pipeline.Processing[entity.Event], error) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.Namespace,
		Name:      "late_data_total",
		Help:      "Late data counter by event name and day.",
	}, []string{"name", "event_day"})

	err := registry.Register(counter)
	if err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	ret := CountLateData{
		counter: counter,
		clock:   clock,
		inner:   p,
	}

	return ret, nil
}

func (p CountLateData) Process(ctx context.Context, event entity.Event) error {
	err := p.inner.Process(ctx, event)
	if err != nil {
		return err // Count only successfully processed data
	}

	if event.Name == eventNameHostState {
		return nil
	}

	eventTime, err := ExtractEventTime(event)
	if err != nil {
		log.Logger().Error(err, "Failed to extract time to count late data")

		return nil // Not a processing error
	}

	limit := p.computeDeadline()
	if eventTime.After(limit) {
		return nil
	}

	eventDayStr := p.computeEventDayLabel(eventTime)
	p.counter.WithLabelValues(event.Name, eventDayStr).Inc()

	return nil
}

// CCX processes data of the previous day twice per day. Last time at 2PM.
// Therefore before 2PM, data from previous day are not late yet.
// But after 2PM, only data of the current day will be processed.
func (p CountLateData) computeDeadline() time.Time {
	now := p.clock.Now().UTC()
	lastCCXProcessingTime := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, time.UTC)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if now.After(lastCCXProcessingTime) {
		return today
	}

	return today.Add(-24 * time.Hour)
}

func (p CountLateData) computeEventDayLabel(eventTime time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d", eventTime.Year(), eventTime.Month(), eventTime.Day())
}
