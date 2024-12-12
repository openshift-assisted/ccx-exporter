package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	kafkaURL      = "localhost:32323"
	valkeyURL     = "localhost:30379"
	localstackURL = "http://localhost:31566"

	maxSizeName = 12
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

func CreateTestContext(conf TestConfig) (TestContext, error) {
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
	saramaConfig.Version = sarama.V3_6_0_0
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

	return ret, nil
}

// Generic func

func (tc TestContext) DeployAll(ctx context.Context) error {
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

	return nil
}

func (tc TestContext) Shutdown(ctx context.Context) error {
	// Disconnect client first
	err := tc.Close(ctx)
	if err != nil {
		return err
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
	err := runMakefileCommand(
		"local.processing",
		map[string]string{
			"DEPLOYMENT_NAME": tc.Config.DeploymentName,
			"VALKEY_URL":      tc.Config.ValkeyURL,
			"DQL_S3_BUCKET":   tc.Config.DLQS3Bucket,
			"KAFKA_TOPIC":     tc.Config.KafkaTopic,
			"S3_BUCKET":       tc.Config.OutputS3Bucket,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy processing: %w", err)
	}

	return nil
}

func (tc TestContext) DeleteProcessing(ctx context.Context) error {
	err := runMakefileCommand(
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

func (tc TestContext) deleteS3Bucket(ctx context.Context, bucket string) error {
	_, err := tc.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &bucket})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucket, err)
	}

	return nil
}
