package e2e_test

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

var _ = Describe("Checking infraenv happy path", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("hp-infraenv")

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

	When("pushing an valid infra_env event", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/input/infra_env.kafka.json")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should process the event", func(ctx SpecContext) {
			By("eventually creating a file in s3 (result) with expected output")
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.OutputS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
				g.Expect(objects[0]).To(ContainSubstring(e2e.S3Path(e2e.EventTypeInfraEnvs, e2e.EventDate)))

				// Parsing actual output
				actualContent, err := testContext.GetS3Object(ctx, objects[0])
				g.Expect(err).NotTo(HaveOccurred())

				// Parsing expected output
				expectedContent, err := os.ReadFile("resources/output/infra_env.s3.json")
				g.Expect(err).NotTo(HaveOccurred())

				// Check it matches
				g.Expect(actualContent).To(MatchJSON(expectedContent))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("eventually incrementing the data count metrics")
			Eventually(func(g Gomega, ctx context.Context) {
				metric, err := testContext.GetMetric(ctx, e2e.DataCountMetricFamily, e2e.KeyValue{Key: "name", Value: "InfraEnv"})
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(metric.Counter).NotTo(BeNil())
				g.Expect(metric.Counter.Value).NotTo(BeNil())
				g.Expect(*metric.Counter.Value).To(BeEquivalentTo(1))
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})
})
