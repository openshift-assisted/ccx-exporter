package e2e_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

var _ = Describe("Checking late data handling", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("latedata")

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

	When("pushing event data from 2024-10-27", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/input/old_event.json")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed but report late data", func(ctx SpecContext) {
			By("eventually creating a file in s3 (result)")
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.OutputS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
				g.Expect(objects[0]).To(ContainSubstring(".events/2024-10-27"))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("eventually incrementing the late metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metric, err := testContext.GetMetric(ctx, e2e.LateDataMetricFamily, e2e.KeyValue{Key: "event_day", Value: "2024-10-27"}, e2e.KeyValue{Key: "name", Value: "Event"})
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(metric.Counter).NotTo(BeNil())
				g.Expect(metric.Counter.Value).NotTo(BeNil())
				g.Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})
})
