package config

import "time"

type Config struct {
	GracefulDuration time.Duration
	Metrics          Metrics
	Logs             Logs
	DeadLetterQueue  S3
	Kafka            Kafka
	Valkey           Valkey
	Output           Output
}

type Metrics struct {
	Port int
}

type Logs struct {
	Level   int
	Encoder EncoderType
}

type EncoderType string

const (
	EncoderTypeJson    EncoderType = "json"
	EncoderTypeConsole EncoderType = "console"
)

type Output struct {
	S3 []S3
}

type S3 struct {
	SecretPath string

	Bucket       string `secret:"bucket"`
	KeyPrefix    string
	BaseEndpoint string `secret:"endpoint"`
	Region       string `secret:"aws_region"`
	UsePathStyle bool
	Creds        AWSCreds
}

type AWSCreds struct {
	AccessKeyID     string `secret:"aws_access_key_id"`
	SecretAccessKey string `secret:"aws_secret_access_key"`
}

func (c AWSCreds) String() string {
	if c.AccessKeyID != "" && c.SecretAccessKey != "" {
		return "creds set"
	}

	return "no creds"
}

type Kafka struct {
	Broker   KafkaBroker
	Consumer KafkaConsumer
}

type KafkaBroker struct {
	URLs    string
	Version string
	Creds   KafkaCreds
	UseTLS  bool
}

type KafkaCreds struct {
	User               string
	Password           string
	UseSCRAMSHA512Auth bool
}

func (c KafkaCreds) String() string {
	if !c.UseSCRAMSHA512Auth {
		return "not using SCRAM auth"
	}

	switch {
	case c.User != "" && c.Password != "":
		return "creds are set"
	case c.User != "":
		return "only user is set"
	case c.Password != "":
		return "only password is set"
	default:
		return "no creds"
	}
}

type KafkaConsumer struct {
	Topic string
	Group string
}

type Valkey struct {
	URL   string
	TTL   time.Duration
	Creds ValkeyCreds
}

type ValkeyCreds struct {
	Password string
}

func (c ValkeyCreds) String() string {
	if c.Password != "" {
		return "password set"
	}

	return "no password"
}
