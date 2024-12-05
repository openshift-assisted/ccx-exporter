package pipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

				Expect(err).To(HaveOccurred(), "nil error")

				Expect(errors.Is(err, pipeline.ErrRetryableError)).To(BeTrue(), "error is retryable")

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
				Expect(errors.Is(err, errOneError)).To(BeTrue(), "error is the original error")
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

				Expect(errors.Is(err, pipeline.ErrRetryableError)).To(BeTrue(), "error is retryable")

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
				Expect(errors.Is(err, errOneError)).To(BeTrue(), "error is the original error")
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
			Expect(errors.Is(err, err1) || errors.Is(err, err2)).To(BeTrue(), "error is one of the 2 errors")
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
				Expect(errors.Is(err, errOneError)).To(BeTrue(), "error is the original error")
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

		When("the inner processing continuously fails", func() {
			Context("With a generic error", func() {
				BeforeEach(func() {
					proc.EXPECT().Process(gomock.Any(), data).Return(errOneError).Times(1)
				})
				It("should fail immediatly", func(ctx SpecContext) {
					err := retry.Process(ctx, data)
					Expect(err).To(HaveOccurred(), "non nil error")
					Expect(errors.Is(err, errOneError)).To(BeTrue(), "error is the original error")
				})
			})

			Context("With a retryable ErrProcessingError", func() {
				BeforeEach(func() {
					proc.EXPECT().Process(gomock.Any(), data).Return(errRetryableErrProcessingError).Times(3)
				})

				It("should return a retryable ErrProcessingError", func(ctx SpecContext) {
					err := retry.Process(ctx, data)

					Expect(err).To(HaveOccurred(), "nil error")

					Expect(errors.Is(err, pipeline.ErrRetryableError)).To(BeTrue(), "error is retryable")

					processingError := pipeline.ErrProcessingError{}
					Expect(errors.As(err, &processingError)).To(BeTrue(), "error is a ErrProcessingError")
					Expect(processingError.Category).To(Equal(oneCategory), "ErrProcessingError category is preserved")
				})
			})
		})
	})
})
