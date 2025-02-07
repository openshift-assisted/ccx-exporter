package processing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
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

	// Rename & "jsonify" inventory -> host_inventory
	inventory, err := computeHostInventory(payload["inventory"])
	if err != nil {
		return err
	}

	payload["host_inventory"] = inventory
	delete(payload, "inventory")

	// Drop free_addresses (not push by scraper and quite big for not that much value)
	delete(payload, "free_addresses")

	// Create HostState
	hostState := entity.HostState{
		ClusterID: clusterID,
		HostID:    hostID,
		Metadata:  CopyPayload(event.Metadata),
		Payload:   payload,
	}

	// Store
	err = m.hostRepo.WriteHostState(ctx, hostState)
	if err != nil {
		return fmt.Errorf("failed to write host state: %w", err)
	}

	return nil
}

func computeHostInventory(input interface{}) (interface{}, error) {
	if input == nil {
		return nil, nil
	}

	inventory, err := castInventory(input)
	if err != nil {
		return nil, err
	}

	if inventory == nil {
		return nil, nil
	}

	ret := make(map[string]interface{})

	err = json.Unmarshal(inventory, &ret)
	if err != nil {
		return nil, common.NewErrProcessingError(err, categoryErrInvalidHostEvent, nil, "failed to unmarshal inventory")
	}

	return ret, nil
}

func castInventory(inventory interface{}) ([]byte, error) {
	inventoryStr, ok := inventory.(string)
	if ok {
		return []byte(inventoryStr), nil
	}

	inventoryByteA, ok := inventory.([]byte)
	if ok {
		return inventoryByteA, nil
	}

	return nil, pipeline.NewErrProcessingError(fmt.Errorf("unexpected type for inventory"), categoryErrInvalidHostEvent, nil)
}
