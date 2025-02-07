package processing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

const (
	categoryErrInvalidClusterState = "invalid_cluster_state"
	categoryErrHostWriterRepo      = "host_writer_repo"
)

func (m Main) processClusterState(ctx context.Context, event entity.Event) error {
	payload := CopyPayload(event.Payload)

	// Extract clusterID
	clusterID, err := ExtractString(event.Payload, "id")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, nil, "failed to extract id")
	}

	// Get HostState for cluster
	// Sorted by host id to have a deterministic cluster_state_id
	hostStates, err := m.hostRepo.GetHostStates(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get host states: %w", err)
	}

	sort.Slice(hostStates, func(i, j int) bool {
		return hostStates[i].HostID < hostStates[j].HostID
	})

	hosts := make([]interface{}, 0, len(hostStates))
	for _, hs := range hostStates {
		hosts = append(hosts, hs.Payload)
	}

	payload["hosts"] = hosts

	// Check Mandatory fields (created_at, updated_at, email_domain)
	_, err = ExtractString(event.Payload, "created_at")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "failed to extract created_at")
	}

	updatedAtStr, err := ExtractString(event.Payload, "updated_at")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "failed to extract updated_at")
	}

	_, err = ExtractString(event.Payload, "email_domain")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "failed to extract email_domain")
	}

	// Validate date
	updatedAt, err := ValidateDate(updatedAtStr)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "invalid updated_at")
	}

	payload["updated_at"] = FormatDate(updatedAt)

	// Anonymize user_name
	hashedUser, err := HashValue(payload, "user_name")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "failed to hash user_name")
	}

	payload["user_id"] = hashedUser
	delete(payload, "user_name")

	// Compute cluster_state_id
	clusterStateID, err := HashPayload(event.Payload)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidClusterState, m.makeInputsFromHostStates(hostStates), "failed to compute cluster state id")
	}

	payload["cluster_state_id"] = clusterStateID

	// Create ClusterState
	clusterState := entity.ProjectedClusterState{
		ID:        clusterStateID,
		Timestamp: updatedAt,
		Payload:   payload,
	}

	// Store
	err = m.projectionWriter.WriteProjectedClusterState(ctx, clusterState)
	if err != nil {
		// If the error is already a processing error, keep the category and add the host states as additional inputs
		inputs := m.makeInputsFromHostStates(hostStates)
		category := categoryErrHostWriterRepo
		pErr := pipeline.ErrProcessingError{}

		if errors.As(err, &pErr) {
			inputs = append(inputs, pErr.AdditionalInputs...)
			category = pErr.Category
		}

		return common.NewErrProcessingError(err, category, inputs, "failed to write cluster state")
	}

	return nil
}

func (m Main) makeInputsFromHostStates(states []entity.HostState) []pipeline.Input {
	logger := log.Logger()

	ret := make([]pipeline.Input, 0, len(states))

	for _, state := range states {
		stateStr, err := json.Marshal(state)
		if err != nil {
			logger.Error(err, "failed to marshal host state")

			continue
		}

		ret = append(ret, pipeline.Input{
			Source: "host_repo",
			Key:    fmt.Sprintf("%s-%s", state.ClusterID, state.HostID),
			Value:  stateStr,
		})
	}

	return ret
}
