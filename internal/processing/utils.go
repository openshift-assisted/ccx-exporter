package processing

import (
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

func ValidateDate(date string) error {
	_, err := time.Parse("2006-01-02T15:04:05.000Z", date)
	if err != nil {
		return fmt.Errorf("failed to parse time: %w", err)
	}

	return nil
}
