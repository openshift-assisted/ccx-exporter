package factory

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
)

func CreateKafkaConsumer(kafkaConfig config.Kafka) (sarama.ConsumerGroup, error) {
	conf := sarama.NewConfig()

	// mandatory configuration
	conf.Consumer.Offsets.AutoCommit.Enable = true
	conf.Consumer.Return.Errors = true

	// initial offset
	conf.Consumer.Offsets.Initial = sarama.OffsetOldest

	// clientID
	conf.ClientID = computeClientID(kafkaConfig.Consumer.Group)

	// kafka version
	version, err := sarama.ParseKafkaVersion(kafkaConfig.Broker.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kafka version: %w", err)
	}

	conf.Version = version

	// Kafka URLs
	urls := strings.Split(kafkaConfig.Broker.URLs, ",")

	// Kafka auth
	if kafkaConfig.Broker.Creds.UseSCRAMSHA512Auth && kafkaConfig.Broker.Creds.User != "" && kafkaConfig.Broker.Creds.Password != "" {
		conf.Net.SASL.Enable = true
		conf.Net.SASL.User = kafkaConfig.Broker.Creds.User
		conf.Net.SASL.Password = kafkaConfig.Broker.Creds.Password

		conf.Net.SASL.SCRAMClientGeneratorFunc = GetXDGSCRAM512Client
		conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	}

	if kafkaConfig.Broker.UseTLS {
		conf.Net.TLS.Enable = true
	}

	// kafka consumer group
	ret, err := sarama.NewConsumerGroup(urls, kafkaConfig.Consumer.Group, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer group: %w", err)
	}

	return ret, nil
}

func computeClientID(groupID string) string {
	prefix, err := os.Hostname()
	if err != nil {
		prefix = fmt.Sprintf("clientid-%v", groupID)
	}

	return fmt.Sprintf("%s-%x", prefix, rand.Int31())
}

// SCRAM client

func GetXDGSCRAM256Client() sarama.SCRAMClient {
	return &KafkaXDGSCRAMClient{HashGeneratorFcn: sha256inst}
}

func GetXDGSCRAM512Client() sarama.SCRAMClient {
	return &KafkaXDGSCRAMClient{HashGeneratorFcn: sha512inst}
}

var (
	sha256inst scram.HashGeneratorFcn = sha256.New
	sha512inst scram.HashGeneratorFcn = sha512.New
)

type KafkaXDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *KafkaXDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}

	x.ClientConversation = x.Client.NewConversation()

	return nil
}

func (x *KafkaXDGSCRAMClient) Step(challenge string) (string, error) {
	return x.ClientConversation.Step(challenge)
}

func (x *KafkaXDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
