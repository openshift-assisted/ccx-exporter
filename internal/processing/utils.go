package processing

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

var (
	errMissingKey       = errors.New("missing key")
	errFieldInvalidType = errors.New("field type was not the expected one")
	errEmptyValue       = errors.New("empty value")
)

func ExtractEventTime(event entity.Event) (time.Time, error) {
	return time.Time{}, errors.New("not implemented")
}

func ExtractString(payload map[string]interface{}, key string) (string, error) {
	value, present := payload[key]
	if !present {
		return "", errMissingKey
	}

	ret, ok := value.(string)
	if !ok {
		return "", errFieldInvalidType
	}

	if ret == "" {
		return "", errEmptyValue
	}

	return ret, nil
}

func ValidateDate(date string) (time.Time, error) {
	ret, err := time.Parse("2006-01-02T15:04:05.000Z", date)
	if err != nil {
		return ret, fmt.Errorf("failed to parse time: %w", err)
	}

	return ret, nil
}

func CopyPayload(payload map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})

	for k, v := range payload {
		ret[k] = v
	}

	return ret
}

func HashValue(payload map[string]interface{}, key string) (string, error) {
	value, err := ExtractString(payload, key)
	if err != nil {
		return "", fmt.Errorf("failed to extract string: %w", err)
	}

	hash := md5.New()

	_, err = hash.Write([]byte(value))
	if err != nil {
		return "", fmt.Errorf("failed to hash value: %w", err)
	}

	hashBytes := hash.Sum(nil)
	ret := hex.EncodeToString(hashBytes[:])

	return ret, nil
}
