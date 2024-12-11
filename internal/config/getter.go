package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const prefix = "CCXEXPORTER"

// Parse reads the configuration file given as parameter.
func Parse(confFile string) (*Config, error) {
	ret := Config{}

	setDefault()

	viper.SetEnvPrefix(prefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	if len(confFile) > 0 {
		viper.SetConfigFile(confFile)

		err := viper.ReadInConfig()
		if err != nil {
			return &ret, fmt.Errorf("failed to read config file %v: %w", confFile, err)
		}
	}

	err := viper.Unmarshal(&ret)
	if err != nil {
		return &ret, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &ret, nil
}

func setDefault() {
	viper.SetDefault("logs.level", 4)
	viper.SetDefault("logs.encoder", EncoderTypeConsole)
	viper.SetDefault("gracefulDuration", "8s")
	viper.SetDefault("metrics.port", 7777)
	viper.SetDefault("deadletterqueue.region", "us-east-1")
	viper.SetDefault("s3.region", "us-east-1")
}
