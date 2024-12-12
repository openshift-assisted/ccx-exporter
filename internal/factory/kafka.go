package factory

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/IBM/sarama"

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
