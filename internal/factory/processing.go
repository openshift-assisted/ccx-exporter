package factory

import (
	"fmt"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

/*
 * DecorateProcessing decorates the processing as follow:
 *
 * panic --> duration --> retry --> main (anonymize + ... + s3)
 */
func DecorateProcessing(mainProcessing pipeline.Processing[entity.Event], registry prometheus.Registerer) (pipeline.Processing[entity.Event], error) {
	ret := mainProcessing

	ret = pipeline.NewRetryProcessing(ret, pipeline.RetryConfig{})
	ret, err := pipeline.NewDurationMetricsDecoratorProcessing(ret, registry, clockwork.NewRealClock(), pipeline.MetricsConfig{Namespace: "main"})
	if err != nil {
		return nil, fmt.Errorf("failed to create duration metrics processor: %w", err)
	}

	ret = pipeline.NewPanicHandlerProcessing(ret)

	return ret, nil
}

/*
 * DecorateErrorProcessing decorates the error processing as follow:
 *
 *										---> retry --> main (dlq)
 *	panic --> duration --> parallel ---|
 *										---> error count
 */
func DecorateErrorProcessing(mainProcessing pipeline.ErrorProcessing, registry prometheus.Registerer) (pipeline.ErrorProcessing, error) {
	ret := mainProcessing

	ret = pipeline.NewRetryProcessing(ret, pipeline.RetryConfig{})

	errorCount, err := pipeline.NewErrorCountProcessing(registry, pipeline.MetricsConfig{Namespace: "error"})
	if err != nil {
		return nil, fmt.Errorf("failed to create error count processing: %w", err)
	}

	ret = pipeline.NewParallelProcessing(ret, errorCount)

	ret, err = pipeline.NewDurationMetricsDecoratorProcessing(ret, registry, clockwork.NewRealClock(), pipeline.MetricsConfig{Namespace: "error"})
	if err != nil {
		return nil, fmt.Errorf("failed to create duration metrics processor: %w", err)
	}

	ret = pipeline.NewPanicHandlerProcessing(ret)

	return ret, nil
}
