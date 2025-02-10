package processing

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

var (
	errMissingKey       = errors.New("missing key")
	errFieldInvalidType = errors.New("field type was not the expected one")
	errEmptyValue       = errors.New("empty value")

	dateFormat = "<year>-<month>-<day>T<hour>:<minute>:<second>.<micro>Z"
)

func ExtractEventTime(event entity.Event) (time.Time, error) {
	ret := time.Time{}

	var dateStr string
	var err error

	switch event.Name {
	case eventNameClusterState:
		dateStr, err = ExtractString(event.Payload, "updated_at")
	case eventNameEvent:
		dateStr, err = ExtractString(event.Payload, "event_time")
	case eventNameInfraEnvState:
		dateStr, err = ExtractString(event.Payload, "updated_at")
	default:
		return ret, fmt.Errorf("unexpected event name: %s", event.Name)
	}

	if err != nil {
		return ret, fmt.Errorf("failed to extract date time: %w", err)
	}

	ret, err = ValidateDate(dateStr)
	if err != nil {
		return ret, fmt.Errorf("invalid date %s: %w", dateStr, err)
	}

	return ret, nil
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
	ret, err := time.Parse("2006-01-02T15:04:05.9Z", date)
	if err != nil {
		return ret, fmt.Errorf("failed to parse time: %w", err)
	}

	return ret, nil
}

func FormatDate(date time.Time) string {
	replacer := strings.NewReplacer(
		"<year>", fmt.Sprintf("%04d", date.Year()),
		"<month>", fmt.Sprintf("%02d", date.Month()),
		"<day>", fmt.Sprintf("%02d", date.Day()),
		"<hour>", fmt.Sprintf("%02d", date.Hour()),
		"<minute>", fmt.Sprintf("%02d", date.Minute()),
		"<second>", fmt.Sprintf("%02d", date.Second()),
		"<micro>", fmt.Sprintf("%06d", int(date.Nanosecond()/1e3)),
	)

	return replacer.Replace(dateFormat)
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

	return hash([]byte(value))
}

func HashPayload(payload map[string]interface{}) (string, error) {
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	return hash(payloadStr)
}

func hash(value []byte) (string, error) {
	hash := md5.New()

	_, err := hash.Write(value)
	if err != nil {
		return "", fmt.Errorf("failed to hash value: %w", err)
	}

	hashBytes := hash.Sum(nil)
	ret := hex.EncodeToString(hashBytes[:])

	return ret, nil
}
