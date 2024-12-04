package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const prefix = "CCXEXPORTER"

var conf Config

// Parse reads the configuration file given as parameter.
func Parse(confFile string) (*Config, error) {
	setDefault()

	viper.SetEnvPrefix(prefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	if len(confFile) > 0 {
		viper.SetConfigFile(confFile)

		err := viper.ReadInConfig()
		if err != nil {
			return &conf, fmt.Errorf("failed to read config file %v: %w", confFile, err)
		}
	}

	err := viper.Unmarshal(&conf)
	if err != nil {
		return &conf, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &conf, nil
}

// KafkaConfig returns kafka configuration.
// Passwords and sensitive information should be hidden with by implementing Stringer.
func KafkaConfig() Kafka {
	return conf.Kafka
}

func setDefault() {
	viper.SetDefault("logs.level", 4)
	viper.SetDefault("logs.encoder", EncoderTypeConsole)
	viper.SetDefault("defaultTimeout", "8s")
	viper.SetDefault("metrics.port", 7777)
}
