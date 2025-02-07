package e2e_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

var _ = Describe("Checking invalid data handling", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("invalid-paylaod")

		testContext, err = e2e.CreateTestContext(testConfig, kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		err = testContext.DeployAll(ctx)
		Expect(err).NotTo(HaveOccurred())
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

	When("pushing valid json with invalid data", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/input/invalid.json")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report the failure", func(ctx SpecContext) {
			By("eventually creating a file in s3 (dlq)")
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.DLQS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("eventually incrementing the error metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metric, err := testContext.GetMetric(ctx, e2e.ErrorMetricFamily, e2e.KeyValue{Key: "category", Value: "unknown_name"})
				Expect(err).NotTo(HaveOccurred())

				Expect(metric.Counter).NotTo(BeNil())
				Expect(metric.Counter.Value).NotTo(BeNil())
				Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})
})
