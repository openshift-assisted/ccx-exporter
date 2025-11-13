package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
			return nil, fmt.Errorf("failed to read config file %v: %w", confFile, err)
		}
	}

	err := viper.Unmarshal(&ret)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	for i := range ret.Output.S3 {
		err := loadS3Config(&ret.Output.S3[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse s3 config (%d): %w", i, err)
		}
	}

	err = loadS3Config(&ret.DeadLetterQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dlq s3 config: %w", err)
	}

	return &ret, nil
}

func setDefault() {
	viper.SetDefault("logs.level", 4)
	viper.SetDefault("logs.encoder", EncoderTypeConsole)
	viper.SetDefault("gracefulDuration", "8s")
	viper.SetDefault("metrics.port", 7777)
	viper.SetDefault("output.s3", []S3{})
}

func loadS3Config(s3 *S3) error {
	if s3 == nil {
		return errors.New("s3 config can't be nil")
	}

	structValue := reflect.ValueOf(s3).Elem()

	return loadSecretRecursive(s3.SecretPath, structValue)
}

func loadSecretRecursive(mountPath string, structValue reflect.Value) error {
	structType := structValue.Type()

	for i := 0; i < structValue.NumField(); i++ {
		fieldType := structType.Field(i)
		fieldValue := structValue.Field(i)

		// Load recursively for AWSCreds
		if fieldType.Type.Kind() == reflect.Struct {
			err := loadSecretRecursive(mountPath, fieldValue)
			if err != nil {
				return err
			}

			continue
		}

		fileKey := fieldType.Tag.Get("secret")

		if fileKey == "" {
			continue
		}

		filePath := filepath.Join(mountPath, fileKey)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file for key '%s' (%s): %w", fileKey, filePath, err)
		}

		cleanContent := strings.TrimSpace(string(content))

		if fieldValue.Kind() != reflect.String {
			return fmt.Errorf("unexpected error: field %s is not a string", fieldType.Name)
		}

		fieldValue.SetString(cleanContent)
	}

	return nil
}
