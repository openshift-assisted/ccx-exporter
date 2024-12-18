package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	promdto "github.com/prometheus/client_model/go"
	"go.uber.org/mock/gomock"

	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline/mock"
)

// Helper

type Data struct{}

var (
	data = Data{}

	errOneError = errors.New("error for testing purpose")
	oneCategory = "category1"

	errRetryableErrProcessingError = pipeline.NewRetryableErrProcessingError(errOneError, oneCategory, nil)

	panicReason = "my specific reason"
)

type PanicProcessing struct{}

func (p PanicProcessing) Process(ctx context.Context, data Data) error {
	panic(panicReason)
}

type SlowProcessor struct {
	Sleep time.Duration
	Err   error

	clock clockwork.FakeClock
}

func NewSlowProcessor(clock clockwork.FakeClock) *SlowProcessor {
	return &SlowProcessor{clock: clock}
}

func (s *SlowProcessor) Process(ctx context.Context, data Data) error {
	s.clock.Advance(s.Sleep)

	return s.Err
}

func pointer[T any](obj T) *T {
	return &obj
}

func filterMetricByLabel(metrics []*promdto.Metric, labelName, labelValue string) *promdto.Metric {
	for _, metric := range metrics {
		if metric == nil {
			continue
		}

		if len(metric.Label) == 0 {
			continue
		}

		for _, label := range metric.Label {
			if label == nil || label.Name == nil || label.Value == nil {
				continue
			}

			if *label.Name == labelName && *label.Value == labelValue {
				return metric
			}
		}
	}

	return nil
}

// Test

func TestProcessingHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processing helpers test suite")
}

// Test Parallel

var _ = Describe("Testing ParallelProcessing with 2 Processing", func() {
	var ctrl *gomock.Controller

	var parallel pipeline.Processing[Data]
	var proc1, proc2 *mock.MockProcessing[Data]

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		proc1 = mock.NewMockProcessing[Data](ctrl)
		proc2 = mock.NewMockProcessing[Data](ctrl)

		parallel = pipeline.NewParallelProcessing(proc1, proc2)
	})

	When("both processing return nil", func() {
		BeforeEach(func() {
			proc1.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
			proc2.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
		})

		It("should succeed", func(ctx SpecContext) {
			By("calling Process")
			err := parallel.Process(ctx, data)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("only the first processing returns an error", func() {
		Context("and the error is a retryable ErrProcessingError", func() {
			BeforeEach(func() {
				proc1.EXPECT().Process(gomock.Any(), data).Return(errRetryableErrProcessingError).Times(1)
				proc2.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
			})

			It("should return a retryable ErrProcessingError", func(ctx SpecContext) {
				err := parallel.Process(ctx, data)

				Expect(err).To(HaveOccurred(), "non nil error")
				Expect(err).Should(MatchError(pipeline.ErrRetryableError), "error is retryable")

				processingError := pipeline.ErrProcessingError{}
				Expect(errors.As(err, &processingError)).To(BeTrue(), "error is a ErrProcessingError")
				Expect(processingError.Category).To(Equal(oneCategory), "ErrProcessingError category is preserved")
			})
		})

		Context("and the error is generic", func() {
			BeforeEach(func() {
				proc1.EXPECT().Process(gomock.Any(), data).Return(errOneError).Times(1)
				proc2.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
			})

			It("should fail", func(ctx SpecContext) {
				err := parallel.Process(ctx, data)
				Expect(err).To(HaveOccurred(), "non nil error")
				Expect(err).Should(MatchError(errOneError), "error is the original error")
			})
		})
	})

	When("only the second processing returns an error", func() {
		Context("and the error is a retryable ErrProcessingError", func() {
			BeforeEach(func() {
				proc1.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
				proc2.EXPECT().Process(gomock.Any(), data).Return(errRetryableErrProcessingError).Times(1)
			})

			It("should return a retryable ErrProcessingError", func(ctx SpecContext) {
				err := parallel.Process(ctx, data)

				Expect(err).To(HaveOccurred(), "nil error")

				Expect(err).Should(MatchError(pipeline.ErrRetryableError), "error is retryable")

				processingError := pipeline.ErrProcessingError{}
				Expect(errors.As(err, &processingError)).To(BeTrue(), "error is a ErrProcessingError")
				Expect(processingError.Category).To(Equal(oneCategory), "ErrProcessingError category is preserved")
			})
		})

		Context("and the error is generic", func() {
			BeforeEach(func() {
				proc1.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
				proc2.EXPECT().Process(gomock.Any(), data).Return(errOneError).Times(1)
			})

			It("should fail", func(ctx SpecContext) {
				err := parallel.Process(ctx, data)
				Expect(err).To(HaveOccurred(), "non nil error")
				Expect(err).Should(MatchError(errOneError), "error is the original error")
			})
		})
	})

	When("both processing return an error", func() {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		BeforeEach(func() {
			proc1.EXPECT().Process(gomock.Any(), data).Return(err1).MaxTimes(1)
			proc2.EXPECT().Process(gomock.Any(), data).Return(err2).MaxTimes(1)
		})

		It("should return one of the 2 errors", func(ctx SpecContext) {
			err := parallel.Process(ctx, data)
			Expect(err).To(HaveOccurred(), "non nil error")
			Expect(err).Should(Or(MatchError(err1, "err is not err1"), MatchError(err2, "err is not err2")))
		})
	})
})

// Test Panic Processing

var _ = Describe("Testing panic handler processing", func() {
	var ctrl *gomock.Controller

	var panicHandler, proc pipeline.Processing[Data]
	var mockProc *mock.MockProcessing[Data]

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	When("the inner processing panic", func() {
		BeforeEach(func() {
			proc = PanicProcessing{}
			panicHandler = pipeline.NewPanicHandlerProcessing(proc)
		})

		It("should return an error and not panic", func(ctx SpecContext) {
			err := panicHandler.Process(ctx, data)
			Expect(err).To(HaveOccurred(), "non nil err")
			Expect(err.Error()).To(ContainSubstring(panicReason), "contain the panic reason")
		})
	})

	When("the inner processing doesn't panic", func() {
		BeforeEach(func() {
			mockProc = mock.NewMockProcessing[Data](ctrl)
			panicHandler = pipeline.NewPanicHandlerProcessing(mockProc)
		})

		Context("and return an error", func() {
			BeforeEach(func() {
				mockProc.EXPECT().Process(gomock.Any(), data).Return(errOneError).Times(1)
			})

			It("should return the error", func(ctx SpecContext) {
				err := panicHandler.Process(ctx, data)
				Expect(err).To(HaveOccurred(), "non nil error")
				Expect(err).Should(MatchError(errOneError), "error is the original error")
			})
		})

		Context("and return nil", func() {
			BeforeEach(func() {
				mockProc.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
			})

			It("should return nil", func(ctx SpecContext) {
				err := panicHandler.Process(ctx, data)
				Expect(err).NotTo(HaveOccurred(), "nil err")
			})
		})
	})
})

// Test Retry

var _ = Describe("Testing RetryProcessing", func() {
	var ctrl *gomock.Controller

	var retry pipeline.Processing[Data]
	var proc *mock.MockProcessing[Data]

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		proc = mock.NewMockProcessing[Data](ctrl)
	})

	Context("using a retry processing with 3 max attempts and 100ms delay", func() {
		BeforeEach(func() {
			retry = pipeline.NewRetryProcessing(proc, pipeline.RetryConfig{MaxAttempt: 3, Delay: 100 * time.Millisecond})
		})

		When("the inner processing never fail", func() {
			BeforeEach(func() {
				proc.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1)
			})

			It("should succeed", func(ctx SpecContext) {
				err := retry.Process(ctx, data)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the inner processing only fails the first time with a retryable error", func() {
			BeforeEach(func() {
				gomock.InOrder(
					proc.EXPECT().Process(gomock.Any(), data).Return(errRetryableErrProcessingError).Times(1),
					proc.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1),
				)
			})
			It("should succeed", func(ctx SpecContext) {
				err := retry.Process(ctx, data)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the inner processing only fails the first time with a wrapped retryable error", func() {
			BeforeEach(func() {
				gomock.InOrder(
					proc.EXPECT().Process(gomock.Any(), data).Return(fmt.Errorf("wrapping: %w", errRetryableErrProcessingError)).Times(1),
					proc.EXPECT().Process(gomock.Any(), data).Return(nil).Times(1),
				)
			})
			It("should succeed", func(ctx SpecContext) {
				err := retry.Process(ctx, data)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the inner processing continuously fails", func() {
			Context("With a generic error", func() {
				BeforeEach(func() {
					proc.EXPECT().Process(gomock.Any(), data).Return(errOneError).Times(1)
				})
				It("should fail immediatly", func(ctx SpecContext) {
					err := retry.Process(ctx, data)
					Expect(err).To(HaveOccurred(), "non nil error")
					Expect(err).Should(MatchError(errOneError), "error is the original error")
				})
			})

			Context("With a retryable ErrProcessingError", func() {
				BeforeEach(func() {
					proc.EXPECT().Process(gomock.Any(), data).Return(errRetryableErrProcessingError).Times(3)
				})

				It("should return a retryable ErrProcessingError", func(ctx SpecContext) {
					err := retry.Process(ctx, data)

					Expect(err).To(HaveOccurred(), "nil error")

					Expect(err).Should(MatchError(pipeline.ErrRetryableError), "error is retryable")

					processingError := pipeline.ErrProcessingError{}
					Expect(errors.As(err, &processingError)).To(BeTrue(), "error is a ErrProcessingError")
					Expect(processingError.Category).To(Equal(oneCategory), "ErrProcessingError category is preserved")
				})
			})
		})
	})
})

// Test Metric Duration

var _ = Describe("Testing duration metrics decorator", func() {
	var registry *prometheus.Registry
	var metrics pipeline.Processing[Data]
	var proc *SlowProcessor

	BeforeEach(func() {
		registry = prometheus.NewPedanticRegistry()
	})

	Context("using a processing that takes a custom time to process", func() {
		var err error

		BeforeEach(func() {
			fakeClock := clockwork.NewFakeClock()

			proc = NewSlowProcessor(fakeClock)
			metrics, err = pipeline.NewDurationMetricsDecoratorProcessing(proc, registry, fakeClock,
				pipeline.MetricsConfig{
					Namespace: "test",
					Buckets:   []float64{20, 200, 2000},
				},
			)

			Expect(err).NotTo(HaveOccurred())
		})

		When("several messages are successfully processed with different duration", func() {
			BeforeEach(func() {
				proc.Sleep = 5 * time.Millisecond

				for i := 0; i < 3; i++ {
					err = metrics.Process(context.TODO(), data)
					Expect(err).NotTo(HaveOccurred())
				}

				proc.Sleep = 50 * time.Millisecond

				for i := 0; i < 2; i++ {
					err = metrics.Process(context.TODO(), data)
					Expect(err).NotTo(HaveOccurred())
				}

				proc.Sleep = 500 * time.Millisecond

				err = metrics.Process(context.TODO(), data)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should returns the right number in the metrics", func() {
				metrics, err := registry.Gather()
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(1))
				Expect(metrics[0].Metric).To(HaveLen(1))

				metric := metrics[0].Metric[0]

				// label
				By("checking the label")
				Expect(metric.Label).To(HaveLen(1))
				label := metric.Label[0]
				Expect(*label.Name).To(Equal("failed"))
				Expect(*label.Value).To(Equal("false"))

				// Histogram
				By("checking if it's a histogram")
				Expect(metric.Histogram).NotTo(BeNil())

				// Total count
				By("checking the total number of sample in the metric")
				Expect(metric.Histogram.SampleCount).NotTo(BeNil())
				Expect(*metric.Histogram.SampleCount).To(BeEquivalentTo(6))

				// Buckets
				By("checking the different buckets")
				buckets := metric.Histogram.Bucket
				Expect(buckets).To(ConsistOf(
					&promdto.Bucket{UpperBound: pointer[float64](20), CumulativeCount: pointer[uint64](3)},
					&promdto.Bucket{UpperBound: pointer[float64](200), CumulativeCount: pointer[uint64](5)},
					&promdto.Bucket{UpperBound: pointer[float64](2000), CumulativeCount: pointer[uint64](6)},
				))
			})
		})

		When("some messages are successfully processed and some are not", func() {
			BeforeEach(func() {
				proc.Sleep = 2500 * time.Millisecond

				err = metrics.Process(context.TODO(), data)
				Expect(err).NotTo(HaveOccurred())

				proc.Sleep = 50 * time.Millisecond
				proc.Err = errors.New("failed")

				err = metrics.Process(context.TODO(), data)
				Expect(err).To(HaveOccurred())
			})

			It("should returns the right number in the metrics", func() {
				By("checking there are metrics for success and failure")
				metrics, err := registry.Gather()
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(1))
				Expect(metrics[0].Metric).To(HaveLen(2))

				// Success metric
				By("checking the success metric")
				successMetric := filterMetricByLabel(metrics[0].Metric, "failed", "false")
				Expect(successMetric).NotTo(BeNil())

				// Histogram
				Expect(successMetric.Histogram).NotTo(BeNil())

				// Total count
				Expect(successMetric.Histogram.SampleCount).NotTo(BeNil())
				Expect(*successMetric.Histogram.SampleCount).To(BeEquivalentTo(1))

				// Buckets
				successBuckets := successMetric.Histogram.Bucket
				Expect(successBuckets).To(ConsistOf(
					&promdto.Bucket{UpperBound: pointer[float64](20), CumulativeCount: pointer[uint64](0)},
					&promdto.Bucket{UpperBound: pointer[float64](200), CumulativeCount: pointer[uint64](0)},
					&promdto.Bucket{UpperBound: pointer[float64](2000), CumulativeCount: pointer[uint64](0)},
				))

				// Failure metric
				By("checking the success metric")
				failureMetric := filterMetricByLabel(metrics[0].Metric, "failed", "true")
				Expect(failureMetric).NotTo(BeNil())

				// Histogram
				Expect(failureMetric.Histogram).NotTo(BeNil())

				// Total count
				Expect(failureMetric.Histogram.SampleCount).NotTo(BeNil())
				Expect(*failureMetric.Histogram.SampleCount).To(BeEquivalentTo(1))

				// Buckets
				failureBuckets := failureMetric.Histogram.Bucket
				Expect(failureBuckets).To(ConsistOf(
					&promdto.Bucket{UpperBound: pointer[float64](20), CumulativeCount: pointer[uint64](0)},
					&promdto.Bucket{UpperBound: pointer[float64](200), CumulativeCount: pointer[uint64](1)},
					&promdto.Bucket{UpperBound: pointer[float64](2000), CumulativeCount: pointer[uint64](1)},
				))
			})
		})
	})
})

// Test Error Duration

var _ = Describe("Testing error metrics decorator", func() {
	var registry *prometheus.Registry
	var metrics pipeline.Processing[pipeline.ErrProcessingError]
	var err error

	BeforeEach(func() {
		registry = prometheus.NewPedanticRegistry()

		metrics, err = pipeline.NewErrorCountProcessing(registry, pipeline.MetricsConfig{Namespace: "test"})
		Expect(err).NotTo(HaveOccurred())
	})

	When("processing a ErrProcessingError with an empty category", func() {
		BeforeEach(func() {
			err = metrics.Process(context.TODO(), pipeline.NewErrProcessingError(errOneError, "", nil))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a metric with an empty category", func() {
			metrics, err := registry.Gather()
			Expect(err).NotTo(HaveOccurred())
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Metric).To(HaveLen(1))

			metric := metrics[0].Metric[0]

			// label
			By("checking the label")
			Expect(metric.Label).To(HaveLen(1))
			label := metric.Label[0]
			Expect(*label.Name).To(Equal("category"))
			Expect(*label.Value).To(Equal("empty_category"))

			// CounterVec
			By("checking if it's a counter")
			Expect(metric.Counter).NotTo(BeNil())

			// With 1 error
			Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
		})
	})

	When("processing a bunch of errors with different category", func() {
		BeforeEach(func() {
			for i := 0; i < 3; i++ {
				err = metrics.Process(context.TODO(), pipeline.NewErrProcessingError(errOneError, "category1", nil))
				Expect(err).NotTo(HaveOccurred())
			}

			for i := 0; i < 2; i++ {
				err = metrics.Process(context.TODO(), pipeline.NewErrProcessingError(errOneError, "category2", nil))
				Expect(err).NotTo(HaveOccurred())
			}

			err = metrics.Process(context.TODO(), pipeline.NewErrProcessingError(errOneError, "category3", nil))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return metrics for all different categories", func() {
			metrics, err := registry.Gather()
			Expect(err).NotTo(HaveOccurred())
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Metric).To(HaveLen(3))

			for i, category := range []string{"category3", "category2", "category1"} {
				expectedNbError := i + 1

				metric := filterMetricByLabel(metrics[0].Metric, "category", category)

				// CounterVec
				By("checking if it's a counter")
				Expect(metric.Counter).NotTo(BeNil())

				// With 1 error
				Expect(*metric.Counter.Value).To(BeEquivalentTo(expectedNbError))
			}
		})
	})
})
