package projectedevent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

const (
	keyTemplate = "<prefix><eventType>/<year>-<month>-<day>/<id>.ndjson"

	eventTypeEvents    = ".events"
	eventTypeClusters  = ".clusters"
	eventTypeInfraEnvs = ".infra_envs"

	categoryInvalidKey    = "s3_invalid_key"
	categoryInternalError = "s3_internal_error"
	categoryS3ClientError = "s3_client"
)

var (
	rxHexa        = regexp.MustCompile("^[0-9a-f].*")
	errInvalidKey = errors.New("invalid key")
)

type S3Writer struct {
	s3client *s3.Client

	bucket string
	prefix string
}

func NewS3Writer(s3client *s3.Client, bucket string, prefix string) S3Writer {
	return S3Writer{
		s3client: s3client,
		bucket:   bucket,
		prefix:   prefix,
	}
}

func (s S3Writer) WriteProjectedClusterEvent(ctx context.Context, event entity.ProjectedClusterEvent) error {
	return s.putObject(ctx, eventTypeEvents, entity.Projection(event))
}

func (s S3Writer) WriteProjectedClusterState(ctx context.Context, state entity.ProjectedClusterState) error {
	return s.putObject(ctx, eventTypeClusters, entity.Projection(state))
}

func (s S3Writer) WriteProjectedInfraEnv(ctx context.Context, infraEnv entity.ProjectedInfraEnv) error {
	return s.putObject(ctx, eventTypeInfraEnvs, entity.Projection(infraEnv))
}

func (s S3Writer) putObject(ctx context.Context, eventType string, obj entity.Projection) error {
	// Marshal Payload
	b, err := json.Marshal(obj.Payload)
	if err != nil {
		return common.NewErrProcessingError(err, categoryInternalError, nil, "failed to marshal payload")
	}

	// Compute object key
	key, err := s.computeObjectKey(eventType, obj)
	if err != nil {
		return err
	}

	// Write file
	params := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   bytes.NewReader(b),
	}

	_, err = s.s3client.PutObject(ctx, params)
	if err != nil {
		return common.NewErrProcessingError(err, categoryS3ClientError, nil, "failed to put object")
	}

	return nil
}

func (s S3Writer) computeObjectKey(eventType string, obj entity.Projection) (string, error) {
	if !rxHexa.MatchString(obj.ID) {
		return "", common.NewErrProcessingError(errInvalidKey, categoryInvalidKey, nil, "last part of the key doesn't start by 0-9a-z")
	}

	template := strings.NewReplacer(
		"<prefix>", s.prefix,
		"<eventType>", eventType,
		"<year>", fmt.Sprintf("%04d", obj.Timestamp.Year()),
		"<month>", fmt.Sprintf("%02d", obj.Timestamp.Month()),
		"<day>", fmt.Sprintf("%02d", obj.Timestamp.Day()),
		"<id>", obj.ID,
	)

	return template.Replace(keyTemplate), nil
}
