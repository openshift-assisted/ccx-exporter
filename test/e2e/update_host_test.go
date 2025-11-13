package e2e_test

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

var _ = Describe("Checking updated host case", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("updated-host")

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

	When("a host state event has been processed", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/input/host_v1.minimal.json")
			Expect(err).NotTo(HaveOccurred())

			By("eventually incrementing the data count metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metric, err := testContext.GetMetric(ctx, e2e.DataCountMetricFamily, e2e.KeyValue{Key: "name", Value: "HostState"})
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(metric.Counter).NotTo(BeNil())
				g.Expect(metric.Counter.Value).NotTo(BeNil())
				g.Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})

		Context("and the host state is updated", func() {
			BeforeEach(func() {
				err := testContext.PushFile(ctx, "resources/input/host_v2.minimal.json")
				Expect(err).NotTo(HaveOccurred())

				By("eventually incrementing the data count metrics")
				Eventually(func(g Gomega, ctx context.Context) {
					metric, err := testContext.GetMetric(ctx, e2e.DataCountMetricFamily, e2e.KeyValue{Key: "name", Value: "HostState"})
					g.Expect(err).NotTo(HaveOccurred())

					g.Expect(metric.Counter).NotTo(BeNil())
					g.Expect(metric.Counter.Value).NotTo(BeNil())
					g.Expect(*metric.Counter.Value).To(BeEquivalentTo(2))
				}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
			})

			Context("pushing a cluster state event", func() {
				BeforeEach(func() {
					err := testContext.PushFile(ctx, "resources/input/cluster.minimal.json")
					Expect(err).NotTo(HaveOccurred())
				})

				It("should process the event and display the last value only", func() {
					By("eventually incrementing the data count metrics")
					Eventually(func(g Gomega, ctx context.Context) {
						metric, err := testContext.GetMetric(ctx, e2e.DataCountMetricFamily, e2e.KeyValue{Key: "name", Value: "ClusterState"})
						g.Expect(err).NotTo(HaveOccurred())

						g.Expect(metric.Counter).NotTo(BeNil())
						g.Expect(metric.Counter.Value).NotTo(BeNil())
						g.Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
					}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

					By("having the latest version only in hosts")
					Eventually(func(g Gomega, ctx context.Context) {
						for i, bucket := range testConfig.OutputS3Buckets {
							objects, err := testContext.ListS3Objects(ctx, bucket, "")
							g.Expect(err).NotTo(HaveOccurred())
							g.Expect(len(objects)).To(Equal(1))
							g.Expect(objects[0]).To(ContainSubstring(e2e.S3Path(e2e.EventTypeClusters, e2e.EventDate, i)))

							// Parsing actual output
							actualContent, err := testContext.GetS3Object(ctx, bucket, objects[0])
							g.Expect(err).NotTo(HaveOccurred())

							// Parsing expected output
							expectedContent, err := os.ReadFile("resources/output/cluster.minimal.json")
							g.Expect(err).NotTo(HaveOccurred())

							// Check it matches
							g.Expect(actualContent).To(MatchJSON(expectedContent))
						}
					}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
				})
			})
		})
	})
})
