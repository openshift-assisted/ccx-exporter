package e2e_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

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

				metric, err := e2e.GetMetric(metricResp, e2e.ErrorMetricFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(metric.Counter).NotTo(BeNil())
				Expect(metric.Counter.Value).NotTo(BeNil())
				Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
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

				metric, err := e2e.GetMetric(metricResp, e2e.ErrorMetricFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(metric.Counter).NotTo(BeNil())
				Expect(metric.Counter.Value).NotTo(BeNil())
				Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
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
