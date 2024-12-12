package processingerror

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/internal/version"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

const (
	unknownHostname = "<unknown>"

	keyTemplate = "<prefix>/<year>/<month>/<day>/<topic>/<partition>-<offset>.json"
)

var (
	ErrNilEvent = errors.New("nil event")
)

type S3Writer struct {
	s3client *s3.Client

	bucket string
	prefix string

	hostname string
}

func NewS3Writer(s3client *s3.Client, bucket string, prefix string) S3Writer {
	hostname, err := os.Hostname()
	if err != nil {
		log.Logger().Error(err, "failed to get hostname, falling backing to "+unknownHostname)

		hostname = unknownHostname
	}

	return S3Writer{
		s3client: s3client,
		bucket:   bucket,
		prefix:   prefix,
		hostname: hostname,
	}
}

func (r S3Writer) WriteProcessingError(ctx context.Context, pErr pipeline.ErrProcessingError) error {
	// Create ProcessingError
	obj, err := r.createProcessingError(pErr)
	if err != nil {
		return fmt.Errorf("failed to create local model: %w", err)
	}

	// Marshal ProcessingError
	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal local model: %w", err)
	}

	// Compute object key
	key, err := r.computeObjectKey(pErr)
	if err != nil {
		return fmt.Errorf("failed to compute object key: %w", err)
	}

	// Write file
	params := &s3.PutObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
		Body:   bytes.NewReader(b),
	}

	_, err = r.s3client.PutObject(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to write in s3: %w", err)
	}

	return nil
}

func (r S3Writer) createProcessingError(pErr pipeline.ErrProcessingError) (ProcessingError, error) {
	if pErr.Event == nil {
		return ProcessingError{}, ErrNilEvent
	}

	ret := ProcessingError{
		ProcessingContext: ProcessingContext{
			Component: Component{
				Branch:   version.Branch,
				Revision: version.Revision,
			},
			Time: time.Now(),
			Host: r.hostname,
		},
		Sources: Sources{
			Main: Source{
				Topic:     pErr.Event.Topic,
				Partition: pErr.Event.Partition,
				Offset:    pErr.Event.Offset,
				Payload:   pErr.Event.Value,
			},
			Additional: make([]KeyValue, 0, len(pErr.AdditionalInputs)),
		},
		Reason: Reason{
			Category: pErr.Category,
			Error:    pErr.Error(),
		},
	}

	for _, kv := range pErr.AdditionalInputs {
		ret.Sources.Additional = append(ret.Sources.Additional, KeyValue{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}

	return ret, nil
}

func (r S3Writer) computeObjectKey(pErr pipeline.ErrProcessingError) (string, error) {
	if pErr.Event == nil {
		return "", ErrNilEvent
	}

	template := strings.NewReplacer(
		"<prefix>", r.prefix,
		"<year>", fmt.Sprintf("%04d", pErr.Event.Timestamp.Year()),
		"<month>", fmt.Sprintf("%02d", pErr.Event.Timestamp.Month()),
		"<day>", fmt.Sprintf("%02d", pErr.Event.Timestamp.Day()),
		"<topic>", pErr.Event.Topic,
		"<partition>", fmt.Sprintf("%d", pErr.Event.Partition),
		"<offset>", fmt.Sprintf("%d", pErr.Event.Offset),
	)

	return template.Replace(keyTemplate), nil
}
