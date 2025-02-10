package e2e_test

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-assisted/ccx-exporter/test/e2e"
)

var _ = Describe("Checking cluster event idempotency", func() {
	var testConfig e2e.TestConfig
	var testContext e2e.TestContext

	var ctx context.Context

	var firstContent []byte
	var firstTimestamp time.Time

	BeforeEach(func() {
		var err error
		ctx = context.TODO()

		testConfig = e2e.CreateTestConfig("idem-event")

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

	Context("having already processed a valid cluster event", func() {
		BeforeEach(func() {
			err := testContext.PushFile(ctx, "resources/input/event.kafka.json")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega, ctx context.Context) {
				objects, err := testContext.ListS3Objects(ctx, testConfig.OutputS3Bucket, "")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(objects)).To(Equal(1))
				g.Expect(objects[0]).To(ContainSubstring(e2e.S3Path(e2e.EventTypeEvents, e2e.EventDate)))

				obj, err := testContext.S3Client.GetObject(ctx, &s3.GetObjectInput{
					Bucket: &testConfig.OutputS3Bucket,
					Key:    &objects[0],
				})
				g.Expect(err).NotTo(HaveOccurred())

				firstContent, err = io.ReadAll(obj.Body)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(obj.LastModified).NotTo(BeNil())
				firstTimestamp = *obj.LastModified
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})

		When("pushing the same event again", func() {
			BeforeEach(func() {
				err := testContext.PushFile(ctx, "resources/input/event.kafka.json")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reprocess the event and output the same event", func() {
				By("having only 1 key in s3 with a stable content")
				Eventually(func(g Gomega, ctx context.Context) {
					objects, err := testContext.ListS3Objects(ctx, testConfig.OutputS3Bucket, "")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(len(objects)).To(Equal(1))
					g.Expect(objects[0]).To(ContainSubstring(e2e.S3Path(e2e.EventTypeEvents, e2e.EventDate)))

					obj, err := testContext.S3Client.GetObject(ctx, &s3.GetObjectInput{
						Bucket: &testConfig.OutputS3Bucket,
						Key:    &objects[0],
					})
					g.Expect(err).NotTo(HaveOccurred())

					content, err := io.ReadAll(obj.Body)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(content).To(Equal(firstContent))

					g.Expect(obj.LastModified).NotTo(BeNil())
					g.Expect(obj.LastModified.Unix()).To(BeNumerically(">", firstTimestamp.Unix()))
				}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

				By("eventually incrementing the data count metrics")
				Eventually(func(g Gomega, ctx context.Context) {
					metric, err := testContext.GetMetric(ctx, e2e.DataCountMetricFamily, e2e.KeyValue{Key: "name", Value: "Event"})
					g.Expect(err).NotTo(HaveOccurred())

					g.Expect(metric.Counter).NotTo(BeNil())
					g.Expect(metric.Counter.Value).NotTo(BeNil())
					g.Expect(*metric.Counter.Value).To(BeEquivalentTo(2))
				}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())
			})
		})
	})
})
