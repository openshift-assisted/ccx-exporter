package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promdto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

// Helper

const (
	errorMetricFamily    = "error_processing_error_total"
	lateDataMetricFamily = "main_late_data_total"
)

func getMetrics(metrics string, family string) (*promdto.MetricFamily, error) {
	parser := expfmt.TextParser{}

	metricFamilies, err := parser.TextToMetricFamilies(strings.NewReader(metrics))
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	for _, metricFamily := range metricFamilies {
		if metricFamily == nil || metricFamily.Name == nil {
			continue
		}

		if *metricFamily.Name == family {
			return metricFamily, nil
		}
	}

	return nil, errors.New("not found")
}

// Test Case

var _ = Describe("Checking invalid data handling", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("invalid")

		testContext, err = e2e.CreateTestContext(testConfig, kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		err = testContext.DeployAll(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	When("pushing invalid json", func() {
		var metricsPort uint16
		var stopMetricsPortForward chan struct{}

		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/not_even_json.txt")
			Expect(err).NotTo(HaveOccurred())

			port, stopPortForward, err := testContext.PortForwardProcessingMetrics(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			metricsPort = port
			stopMetricsPortForward = stopPortForward
		})

		AfterEach(func() {
			if stopMetricsPortForward != nil {
				close(stopMetricsPortForward)
			}
		})

		It("should report the failure", func(ctx SpecContext) {
			By("eventually creates a file in s3 (dlq)")
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.DLQS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("eventually increments the error metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metricResp, err := testContext.HttpGet(ctx, fmt.Sprintf("http://localhost:%d/metrics", metricsPort))
				Expect(err).NotTo(HaveOccurred())

				metricFamily, err := getMetrics(metricResp, errorMetricFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(metricFamily.Metric).To(HaveLen(1))
				Expect(metricFamily.Metric[0].Counter).NotTo(BeNil())
				Expect(metricFamily.Metric[0].Counter.Value).NotTo(BeNil())
				Expect(*metricFamily.Metric[0].Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	When("pushing valid json with invalid data", func() {
		var metricsPort uint16
		var stopMetricsPortForward chan struct{}

		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/invalid.json")
			Expect(err).NotTo(HaveOccurred())

			port, stopPortForward, err := testContext.PortForwardProcessingMetrics(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			metricsPort = port
			stopMetricsPortForward = stopPortForward
		})

		AfterEach(func() {
			if stopMetricsPortForward != nil {
				close(stopMetricsPortForward)
			}
		})

		It("should report the failure", func(ctx SpecContext) {
			By("eventually creates a file in s3 (dlq)")
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.DLQS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("eventually increments the error metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metricResp, err := testContext.HttpGet(ctx, fmt.Sprintf("http://localhost:%d/metrics", metricsPort))
				Expect(err).NotTo(HaveOccurred())

				metricFamily, err := getMetrics(metricResp, errorMetricFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(metricFamily.Metric).To(HaveLen(1))
				Expect(metricFamily.Metric[0].Counter).NotTo(BeNil())
				Expect(metricFamily.Metric[0].Counter.Value).NotTo(BeNil())
				Expect(*metricFamily.Metric[0].Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	AfterEach(func() {
		// Keep all components if the test failed
		if CurrentSpecReport().Failed() {
			GinkgoLogr.Info("Test failed", "config", testConfig)

			return
		}

		err := testContext.Shutdown(ctx)
		Expect(err).NotTo(HaveOccurred())
	})
})
