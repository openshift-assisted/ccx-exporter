package processing

import (
	"context"
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

const categoryErrInvalidHostEvent = "invalid_host_event"

func (m Main) processHostState(ctx context.Context, event entity.Event) error {
	// Extract Mandatory fields (cluster_id, id)
	clusterID, err := ExtractString(event.Payload, "cluster_id")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidHostEvent, nil, "failed to extract clusterID")
	}

	hostID, err := ExtractString(event.Payload, "id")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidHostEvent, nil, "failed to extract id")
	}

	payload := CopyPayload(event.Payload)

	// Anonymize user_name
	hashedUser, err := HashValue(payload, "user_name")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidHostEvent, nil, "failed to hash user_name")
	}

	payload["user_id"] = hashedUser
	delete(payload, "user_name")

	// Rename inventory -> host_inventory
	inventory := payload["inventory"]

	payload["host_inventory"] = inventory
	delete(payload, "inventory")

	// Create HostState
	hostState := m.createHostState(hostID, clusterID, payload, event.Metadata)

	// Store
	err = m.hostRepo.WriteHostState(ctx, hostState)
	if err != nil {
		return fmt.Errorf("failed to write host state: %w", err)
	}

	return nil
}

func (m Main) createHostState(id string, clusterID string, payload map[string]interface{}, metadata map[string]interface{}) entity.HostState {
	return entity.HostState{
		ClusterID: clusterID,
		HostID:    id,
		Metadata:  metadata,
		Payload:   payload,
	}
}
