package projectedevent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/ptr"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo"
)

const (
	prefixTemplate = "<prefix><eventType>/<year>-<month>-<day>/"
	extension      = ".ndjson"

	eventTypeEvents    = ".events"
	eventTypeClusters  = ".clusters"
	eventTypeInfraEnvs = ".infra_envs"

	categoryInvalidKey    = "s3_invalid_key"
	categoryS3ClientError = "s3_client"
)

var (
	rxHexa        = regexp.MustCompile("^[0-9a-f].*")
	errInvalidKey = errors.New("invalid key")
)

type S3Repo struct {
	s3client   *s3.Client
	s3uploader *manager.Uploader

	bucket string
	prefix string
}

func NewS3Repo(s3client *s3.Client, bucket string, prefix string) S3Repo {
	return S3Repo{
		s3client: s3client,
		s3uploader: manager.NewUploader(s3client, func(u *manager.Uploader) {
			u.PartSize = 25 * 1024 * 1024
			u.Concurrency = 2
		}),

		bucket: bucket,
		prefix: prefix,
	}
}

// Cluster Event

func (s S3Repo) WriteProjectedClusterEvent(ctx context.Context, event entity.ProjectedClusterEvent) error {
	return s.putObject(ctx, eventTypeEvents, entity.Projection(event))
}

func (s S3Repo) ListProjectedClusterEvent(ctx context.Context, filter repo.ListProjectionFilter) ([]entity.ProjectionMeta, error) {
	return s.listProjection(ctx, eventTypeEvents, filter)
}

func (s S3Repo) GetProjectedClusterEvent(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedClusterEvent, error) {
	ret, err := s.getProjection(ctx, eventTypeEvents, meta)

	return entity.ProjectedClusterEvent(ret), err
}

// Cluster State

func (s S3Repo) WriteProjectedClusterState(ctx context.Context, state entity.ProjectedClusterState) error {
	return s.putObject(ctx, eventTypeClusters, entity.Projection(state))
}

func (s S3Repo) ListProjectedClusterState(ctx context.Context, filter repo.ListProjectionFilter) ([]entity.ProjectionMeta, error) {
	return s.listProjection(ctx, eventTypeClusters, filter)
}

func (s S3Repo) GetProjectedClusterState(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedClusterState, error) {
	ret, err := s.getProjection(ctx, eventTypeClusters, meta)

	return entity.ProjectedClusterState(ret), err
}

// Infra Env

func (s S3Repo) WriteProjectedInfraEnv(ctx context.Context, infraEnv entity.ProjectedInfraEnv) error {
	return s.putObject(ctx, eventTypeInfraEnvs, entity.Projection(infraEnv))
}

func (s S3Repo) ListProjectedInfraEnv(ctx context.Context, filter repo.ListProjectionFilter) ([]entity.ProjectionMeta, error) {
	return s.listProjection(ctx, eventTypeInfraEnvs, filter)
}

func (s S3Repo) GetProjectedInfraEnv(ctx context.Context, meta entity.ProjectionMeta) (entity.ProjectedInfraEnv, error) {
	ret, err := s.getProjection(ctx, eventTypeInfraEnvs, meta)

	return entity.ProjectedInfraEnv(ret), err
}

// Internal methods

func (s S3Repo) putObject(ctx context.Context, eventType string, obj entity.Projection) error {
	// Compute object key
	key, err := s.computeObjectKey(eventType, obj.Meta)
	if err != nil {
		return fmt.Errorf("failed to compute object key: %w", err)
	}

	// Write file
	params := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   bytes.NewReader(obj.Payload),
	}

	_, err = s.s3uploader.Upload(ctx, params)
	if err != nil {
		return common.NewErrProcessingError(err, categoryS3ClientError, nil, "failed to upload object")
	}

	return nil
}

func (s S3Repo) listProjection(ctx context.Context, eventType string, filter repo.ListProjectionFilter) ([]entity.ProjectionMeta, error) {
	ret := make([]entity.ProjectionMeta, 0)

	prefix := s.computePrefix(eventType, filter.Date)

	var continuationToken *string

	for {
		listObjectInput := &s3.ListObjectsV2Input{
			Bucket:            &s.bucket,
			Prefix:            &prefix,
			Delimiter:         ptr.String("/"),
			ContinuationToken: continuationToken,
		}

		resp, err := s.s3client.ListObjectsV2(ctx, listObjectInput)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, object := range resp.Contents {
			if object.Key == nil {
				continue
			}

			id := extractProjectionID(*object.Key)

			ret = append(ret, entity.ProjectionMeta{
				ID:        id,
				Timestamp: filter.Date,
			})
		}

		if resp.NextContinuationToken == nil {
			break
		}

		continuationToken = resp.NextContinuationToken
	}

	return ret, nil
}

func (s S3Repo) getProjection(ctx context.Context, eventType string, meta entity.ProjectionMeta) (entity.Projection, error) {
	ret := entity.Projection{
		Meta: meta,
	}

	key, err := s.computeObjectKey(eventType, meta)
	if err != nil {
		return ret, fmt.Errorf("failed to compute object key: %w", err)
	}

	input := &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}

	resp, err := s.s3client.GetObject(ctx, input)
	if err != nil {
		return ret, fmt.Errorf("failed to get object: %w", err)
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ret, fmt.Errorf("failed to read object body: %w", err)
	}

	ret.Payload = b

	return ret, nil
}

func (s S3Repo) computeObjectKey(eventType string, meta entity.ProjectionMeta) (string, error) {
	if !rxHexa.MatchString(meta.ID) {
		return "", common.NewErrProcessingError(errInvalidKey, categoryInvalidKey, nil, "last part of the key doesn't start by 0-9a-z")
	}

	prefix := s.computePrefix(eventType, meta.Timestamp)
	id := fmt.Sprintf("%s%s", meta.ID, extension)

	ret := fmt.Sprintf("%s%s", prefix, id)

	return ret, nil
}

func (s S3Repo) computePrefix(eventType string, date time.Time) string {
	template := strings.NewReplacer(
		"<prefix>", s.prefix,
		"<eventType>", eventType,
		"<year>", fmt.Sprintf("%04d", date.Year()),
		"<month>", fmt.Sprintf("%02d", date.Month()),
		"<day>", fmt.Sprintf("%02d", date.Day()),
	)

	return template.Replace(prefixTemplate)
}

func extractProjectionID(key string) string {
	withoutSuffix := strings.TrimSuffix(key, extension)
	tmp := strings.Split(withoutSuffix, "/")

	return tmp[len(tmp)-1]
}
