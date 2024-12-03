package e2e_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common test suite")
}

var _ = Describe("Checking the happy path", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("happy-path")

		testContext, err = e2e.CreateTestContext(testConfig)
		Expect(err).NotTo(HaveOccurred())

		err = testContext.DeployAll(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	When("pushing some random data", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/initial.json")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should eventually create a file in s3", func(ctx SpecContext) {
			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.OutputS3Bucket, "/")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
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
