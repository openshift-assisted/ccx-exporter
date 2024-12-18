package processing

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

const categoryErrInvalidClusterEvent = "invalid_cluster_event"

func (m Main) processClusterEvent(ctx context.Context, event entity.Event) error {
	// Extract mandatory fields
	clusterID, err := ExtractString(event.Payload, "cluster_id")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterEvent, nil, "failed to extract clusterID")
	}

	eventTime, err := ExtractString(event.Payload, "event_time")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterEvent, nil, "failed to extract event_time")
	}

	// Validate format
	err = ValidateDate(eventTime)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterEvent, nil, "invalid date format")
	}

	// Compute event ID
	message, err := ExtractString(event.Payload, "message")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterEvent, nil, "failed to extract message")
	}

	eventID := m.computeEventID(clusterID, eventTime, message)

	// Create ClusterEvent entity
	clusterEvent, err := m.createClusterEvent(event, eventID)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterEvent, nil, "failed to create cluster event")
	}

	// Call repo
	err = m.projectionWriter.WriteProjectedClusterEvent(ctx, clusterEvent)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

func (m Main) createClusterEvent(event entity.Event, eventID string) (entity.ProjectedClusterEvent, error) {
	ret := entity.ProjectedClusterEvent(event.Payload)

	ret["event_id"] = eventID

	return ret, nil
}

func (m Main) computeEventID(clusterID, eventTime, message string) string {
	key := fmt.Sprintf("%s%s%s", eventTime, clusterID, message)
	hash := md5.Sum([]byte(key))

	return hex.EncodeToString(hash[:])
}
