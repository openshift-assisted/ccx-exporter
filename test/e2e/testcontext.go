package e2e

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	promdto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	kafkaURL      = "localhost:32323"
	valkeyURL     = "localhost:30379"
	localstackURL = "http://localhost:31566"

	maxSizeName = 12

	namespace = "ccx-exporter"
)

type EventType string

const (
	EventTypeEvents    EventType = ".events"
	EventTypeClusters  EventType = ".clusters"
	EventTypeInfraEnvs EventType = ".infra_envs"
)

type TestConfig struct {
	DeploymentName string
	KafkaTopic     string
	ValkeyName     string
	ValkeyURL      string
	OutputS3Bucket string
	DLQS3Bucket    string
}

type TestContext struct {
	Config TestConfig

	s3Client *s3.Client

	kafkaProducer sarama.SyncProducer
	kafkaAdmin    sarama.ClusterAdmin

	kubeConfig *rest.Config
	kubeClient *kubernetes.Clientset

	metricPort       uint16
	closePortForward chan struct{}
}

var random *rand.Rand

func init() {
	now := time.Now()

	random = rand.New(rand.NewSource(now.UnixMilli()))
}

func CreateTestConfig(test string) TestConfig {
	prefix := test
	if len(test) > maxSizeName {
		prefix = test[:maxSizeName]
	}

	name := fmt.Sprintf("%s-%x", prefix, random.Int31())

	return TestConfig{
		DeploymentName: fmt.Sprintf("processing-%s", name),
		KafkaTopic:     name,
		ValkeyName:     fmt.Sprintf("valkey-%s", name),
		ValkeyURL:      fmt.Sprintf("valkey-%s-0.valkey-%s-headless:6379", name, name),
		OutputS3Bucket: fmt.Sprintf("%s-result", name),
		DLQS3Bucket:    fmt.Sprintf("%s-dlq", name),
	}
}

func CreateTestContext(conf TestConfig, kubeConfigPath string) (TestContext, error) {
	ret := TestContext{
		Config: conf,
	}

	// localstack s3 client
	s3Config, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("useless", "useless", "")),
		config.WithBaseEndpoint(localstackURL),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		return ret, fmt.Errorf("failed to create localstack config: %w", err)
	}

	s3Client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	ret.s3Client = s3Client

	// Kafka client
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_7_1_0
	saramaConfig.ClientID = conf.DeploymentName
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll

	kc, err := sarama.NewClient([]string{kafkaURL}, saramaConfig)
	if err != nil {
		return ret, fmt.Errorf("failed to create kafka client: %w", err)
	}

	kp, err := sarama.NewSyncProducerFromClient(kc)
	if err != nil {
		return ret, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	ret.kafkaProducer = kp

	ka, err := sarama.NewClusterAdminFromClient(kc)
	if err != nil {
		return ret, fmt.Errorf("failed to create kafka admin: %w", err)
	}

	ret.kafkaAdmin = ka

	// Kube client
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return ret, fmt.Errorf("failed to cerate kube config: %w", err)
	}

	ret.kubeConfig = kubeConfig

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return ret, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	ret.kubeClient = kubeClient

	return ret, nil
}

// Generic func

func (tc *TestContext) DeployAll(ctx context.Context) error {
	err := tc.DeployValkey(ctx)
	if err != nil {
		return fmt.Errorf("failed to deploy valkey: %w", err)
	}

	err = tc.CreateS3Buckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to create buckets: %w", err)
	}

	err = tc.DeployProcessing(ctx)
	if err != nil {
		return fmt.Errorf("failed to deploy processing: %w", err)
	}

	port, ch, err := tc.PortForwardProcessingMetrics(ctx)
	if err != nil {
		return fmt.Errorf("failed to port forward metric port: %w", err)
	}

	tc.metricPort = port
	tc.closePortForward = ch

	return nil
}

func (tc TestContext) Shutdown(ctx context.Context) error {
	// Disconnect client first
	err := tc.Close(ctx)
	if err != nil {
		return err
	}

	// Stop port forward
	if tc.closePortForward != nil {
		close(tc.closePortForward)
	}

	// Then delete extra resources
	err = tc.DeleteAll(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (tc TestContext) DeleteAll(ctx context.Context) error {
	err := tc.DeleteProcessing(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete processing: %w", err)
	}

	err = tc.DeleteValkey(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete valkey: %w", err)
	}

	err = tc.DeleteS3Buckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete buckets: %w", err)
	}

	return nil
}

func (tc TestContext) Close(ctx context.Context) error {
	err := tc.kafkaProducer.Close()
	if err != nil {
		return fmt.Errorf("failed to close kafka producer: %w", err)
	}

	err = tc.kafkaAdmin.Close()
	if err != nil {
		return fmt.Errorf("failed to close kafka admin: %w", err)
	}

	return nil
}

// Processing func

func (tc TestContext) DeployProcessing(ctx context.Context) error {
	err := tc.createBucketSecrets(ctx)
	if err != nil {
		return err
	}

	err = runMakefileCommand(
		"local.processing",
		map[string]string{
			"DEPLOYMENT_NAME":          tc.Config.DeploymentName,
			"VALKEY_URL":               tc.Config.ValkeyURL,
			"KAFKA_TOPIC":              tc.Config.KafkaTopic,
			"S3_BUCKET_SECRETNAME":     tc.Config.OutputS3Bucket,
			"S3_DLQ_BUCKET_SECRETNAME": tc.Config.DLQS3Bucket,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy processing: %w", err)
	}

	return nil
}

func (tc TestContext) DeleteProcessing(ctx context.Context) error {
	err := tc.deleteBucketSecrets(ctx)
	if err != nil {
		return err
	}

	err = runMakefileCommand(
		"local.delete.processing",
		map[string]string{
			"DEPLOYMENT_NAME": tc.Config.DeploymentName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete processing: %w", err)
	}

	return nil
}

func (tc TestContext) createBucketSecrets(ctx context.Context) error {
	_, err := tc.kubeClient.CoreV1().Secrets(namespace).Create(ctx, createBucketSecret(tc.Config.OutputS3Bucket), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create s3 secret for result: %w", err)
	}

	_, err = tc.kubeClient.CoreV1().Secrets(namespace).Create(ctx, createBucketSecret(tc.Config.DLQS3Bucket), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create s3 secret for dlq: %w", err)
	}

	return nil
}

func (tc TestContext) deleteBucketSecrets(ctx context.Context) error {
	err := tc.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, tc.Config.OutputS3Bucket, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete s3 secret for result: %w", err)
	}

	err = tc.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, tc.Config.DLQS3Bucket, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete s3 secret for dlq: %w", err)
	}

	return nil
}

func createBucketSecret(bucket string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket,
		},
		StringData: map[string]string{
			"aws_access_key_id":     "useless",
			"aws_secret_access_key": "useless",
			"aws_region":            "us-east-1",
			"bucket":                bucket,
			"endpoint":              "http://localstack:4566",
		},
	}
}

// Valkey func

func (tc TestContext) DeployValkey(ctx context.Context) error {
	err := runMakefileCommand(
		"local.valkey.e2e",
		map[string]string{
			"VALKEY_NAME": tc.Config.ValkeyName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy valkey: %w", err)
	}

	return nil
}

func (tc TestContext) DeleteValkey(ctx context.Context) error {
	err := runMakefileCommand(
		"local.delete.valkey.e2e",
		map[string]string{
			"VALKEY_NAME": tc.Config.ValkeyName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete valkey: %w", err)
	}

	return nil
}

// Kafka func

func (tc TestContext) DeleteKafkaTopic(ctx context.Context) error {
	err := tc.kafkaAdmin.DeleteTopic(tc.Config.KafkaTopic)
	if err != nil {
		return fmt.Errorf("failed to delete kafka topic: %w", err)
	}

	return nil
}

func (tc TestContext) PushFile(ctx context.Context, path string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	_, _, err = tc.kafkaProducer.SendMessage(&sarama.ProducerMessage{
		Topic: tc.Config.KafkaTopic,
		Value: sarama.ByteEncoder(payload),
	})
	if err != nil {
		return fmt.Errorf("failed to push msg: %w", err)
	}

	return nil
}

func (tc TestContext) UpdateDateAndPush(ctx context.Context, path string) error {
	updatedPayload, err := ReadAndUpdateDate(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to read & update file: %w", err)
	}

	_, _, err = tc.kafkaProducer.SendMessage(&sarama.ProducerMessage{
		Topic: tc.Config.KafkaTopic,
		Value: sarama.ByteEncoder(updatedPayload),
	})
	if err != nil {
		return fmt.Errorf("failed to push msg: %w", err)
	}

	return nil
}

// Template
func ReadAndUpdateDate(ctx context.Context, path string) ([]byte, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	now := time.Now()
	nowStr := fmt.Sprintf("%04d-%02d-%02dT00:00:00.000Z", now.Year(), now.Month(), now.Day())

	ret := strings.ReplaceAll(string(payload), "CURRENT_DATE_PLACEHOLDER", nowStr)

	return []byte(ret), nil
}

// S3 func

func (tc TestContext) CreateS3Buckets(ctx context.Context) error {
	err := tc.createS3Bucket(ctx, tc.Config.OutputS3Bucket)
	if err != nil {
		return fmt.Errorf("failed to create output s3 bucket: %w", err)
	}

	err = tc.createS3Bucket(ctx, tc.Config.DLQS3Bucket)
	if err != nil {
		return fmt.Errorf("failed to create dlq s3 bucket: %w", err)
	}

	return nil
}

func (tc TestContext) DeleteS3Buckets(ctx context.Context) error {
	err := tc.deleteS3Bucket(ctx, tc.Config.OutputS3Bucket)
	if err != nil {
		return fmt.Errorf("failed to delete output s3 bucket: %w", err)
	}

	err = tc.deleteS3Bucket(ctx, tc.Config.DLQS3Bucket)
	if err != nil {
		return fmt.Errorf("failed to delete dlq s3 bucket: %w", err)
	}

	return nil
}

func (tc TestContext) ListS3Objects(ctx context.Context, bucket string, prefix string) ([]string, error) {
	ret := make([]string, 0)

	resp, err := tc.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	})
	if err != nil {
		return ret, fmt.Errorf("failed to list object: %w", err)
	}

	for _, obj := range resp.Contents {
		ret = append(ret, *obj.Key)
	}

	return ret, nil
}

func (tc TestContext) createS3Bucket(ctx context.Context, bucket string) error {
	_, err := tc.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &bucket})
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
	}

	return nil
}

func (tc TestContext) deleteS3Bucket(_ context.Context, bucket string) error {
	// Unfortunately the s3 client can't be used because:
	// - the bucket needs to be empty to be deleted
	// - DeleteObject(s) doesn't support PathStyle
	// - PathStyle is mandatory with localstack + kind

	err := runCommand(
		fmt.Sprintf("aws --endpoint-url=%s s3 rb s3://%s --force", localstackURL, bucket),
		[]string{"AWS_ACCESS_KEY_ID=set", "AWS_SECRET_ACCESS_KEY=set"},
	)
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

func S3Path(eventType EventType, date time.Time) string {
	return fmt.Sprintf(
		"ccx-exporter/output/%s/%04d-%02d-%02d/",
		eventType,
		date.Year(), date.Month(), date.Day(),
	)
}

func (tc TestContext) GetS3Object(ctx context.Context, key string) ([]byte, error) {
	obj, err := tc.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &tc.Config.OutputS3Bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", key, err)
	}

	defer obj.Body.Close()

	ret, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object %s: %w", key, err)
	}

	return ret, nil
}

// Kube func
func (tc TestContext) PortForward(ctx context.Context, namespace string, pod string, ports []string) ([]portforward.ForwardedPort, chan struct{}, error) {
	url := tc.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(tc.kubeConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	errChan := make(chan error)

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to port forward into pod: %w", err)
	}

	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return nil, nil, fmt.Errorf("failed to run port forward: %w", err)
	case <-readyChan:
		break
	}

	ret, err := pf.GetPorts()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get used ports: %w", err)
	}

	return ret, stopChan, nil
}

func (tc TestContext) ListProcessingPod(ctx context.Context) (*corev1.PodList, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": tc.Config.DeploymentName,
		},
	}

	label, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to create label selector: %w", err)
	}

	ret, err := tc.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: label.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pod: %w", err)
	}

	return ret, nil
}

func (tc TestContext) PortForwardProcessingMetrics(ctx context.Context) (uint16, chan struct{}, error) {
	podList, err := tc.ListProcessingPod(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to list processing pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return 0, nil, fmt.Errorf("no pods found")
	}

	ports, cancel, err := tc.PortForward(ctx, "ccx-exporter", podList.Items[0].Name, []string{":7777"})
	if err != nil {
		return 0, nil, fmt.Errorf("fail to port forward: %w", err)
	}

	if len(ports) == 0 {
		close(cancel)

		return 0, nil, fmt.Errorf("0 returned port")
	}

	return ports[0].Local, cancel, nil
}

// HTTP func

func (tc TestContext) HttpGet(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp.Body, nil
}

// Metrics

const (
	ErrorMetricFamily     = "error_processing_error_total"
	LateDataMetricFamily  = "processing_late_data_total"
	DataCountMetricFamily = "processing_data_total"
)

type KeyValue struct {
	Key   string
	Value string
}

func (tc TestContext) GetMetric(ctx context.Context, family string, labels ...KeyValue) (*promdto.Metric, error) {
	metricsResp, err := tc.HttpGet(ctx, fmt.Sprintf("http://localhost:%d/metrics", tc.metricPort))
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	defer metricsResp.Close()

	parser := expfmt.TextParser{}

	metricFamilies, err := parser.TextToMetricFamilies(metricsResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	for _, metricFamily := range metricFamilies {
		if metricFamily == nil || metricFamily.Name == nil || len(metricFamily.Metric) == 0 {
			continue
		}

		if *metricFamily.Name != family {
			continue
		}

		for _, metric := range metricFamily.Metric {
			if metricHasAllLabels(metric, labels...) {
				return metric, nil
			}
		}
	}

	return nil, fmt.Errorf("metric %s %+v not found", family, labels)
}

func metricHasAllLabels(metric *promdto.Metric, filters ...KeyValue) bool {
	for _, filter := range filters {
		if !metricHasLabel(metric, filter) {
			return false
		}
	}

	return true
}

func metricHasLabel(metric *promdto.Metric, filter KeyValue) bool {
	for _, label := range metric.Label {
		if label.GetName() == filter.Key && label.GetValue() == filter.Value {
			return true
		}
	}

	return false
}
