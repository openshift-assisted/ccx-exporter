package processing

import (
	"context"
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

const categoryErrInvalidInfraEnvEvent = "invalid_infraenv_event"

func (m Main) processInfraEnv(ctx context.Context, event entity.Event) error {
	// Check updated_at
	updatedAtStr, err := ExtractString(event.Payload, "updated_at")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidInfraEnvEvent, nil, "failed to extract updated_at")
	}

	updatedAt, err := ValidateDate(updatedAtStr)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidInfraEnvEvent, nil, "invalid format for updated_at")
	}

	payload := CopyPayload(event.Payload)
	payload["updated_at"] = FormatDate(updatedAt)

	// Anonymize user_name
	hashedUser, err := HashValue(payload, "user_name")
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidInfraEnvEvent, nil, "failed to hash user_name")
	}

	if hashedUser != "" {
		payload["user_id"] = hashedUser
	}

	delete(payload, "user_name")

	// Add infraenv_state_id
	infraEnvStateID, err := HashPayload(event.Payload)
	if err != nil {
		return common.NewErrProcessingError(err, categoryErrInvalidInfraEnvEvent, nil, "failed to compute infraenv state id")
	}

	payload["infraenv_state_id"] = infraEnvStateID

	// Create Projection
	infraEnv := entity.ProjectedInfraEnv{
		ID:        infraEnvStateID,
		Timestamp: updatedAt,
		Payload:   payload,
	}

	// Store
	err = m.projectionWriter.WriteProjectedInfraEnv(ctx, infraEnv)
	if err != nil {
		return fmt.Errorf("failed to write infraenv: %w", err)
	}

	return nil
}
