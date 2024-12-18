package host

import (
	"context"
	"encoding/json"
	"errors"
	"syscall"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

const (
	categoryInternalError     = "valkey_internal_error"
	categoryValkeyClientError = "valkey_client"
)

type ValkeyRepo struct {
	client     valkey.Client
	expiration time.Duration
}

func NewValkeyRepo(client valkey.Client, expiration time.Duration) ValkeyRepo {
	return ValkeyRepo{
		client:     client,
		expiration: expiration,
	}
}

func (r ValkeyRepo) WriteHostState(ctx context.Context, event entity.HostState) error {
	// Convert to local model
	state := mapToModels(event)

	// Marshal local model
	data, err := json.Marshal(state)
	if err != nil {
		return common.NewErrProcessingError(err, categoryInternalError, nil, "failed to marshal data")
	}

	// Set property
	command := r.client.B().Hset().Key(event.ClusterID).FieldValue().FieldValue(event.HostID, string(data)).Build()

	err = r.client.Do(ctx, command).Error()
	if err != nil {
		switch {
		case r.isRetryable(err):
			return common.NewRetryableErrProcessingError(err, categoryValkeyClientError, nil, "failed to set hkey")
		default:
			return common.NewErrProcessingError(err, categoryValkeyClientError, nil, "failed to set hkey")
		}
	}

	// Set expiration
	expireCommand := r.client.B().Expire().Key(event.ClusterID).Seconds(int64(r.expiration.Seconds())).Build()

	err = r.client.Do(ctx, expireCommand).Error()
	if err != nil {
		switch {
		case r.isRetryable(err):
			return common.NewRetryableErrProcessingError(err, categoryValkeyClientError, nil, "failed to set expiration")
		default:
			return common.NewErrProcessingError(err, categoryValkeyClientError, nil, "failed to set expiration")
		}
	}

	return nil
}

func (r ValkeyRepo) GetHostStates(ctx context.Context, clusterID string) ([]entity.HostState, error) {
	command := r.client.B().Hgetall().Key(clusterID).Build()

	resp := r.client.Do(ctx, command)

	err := resp.Error()
	if err != nil {
		switch {
		case r.isRetryable(err):
			return nil, common.NewRetryableErrProcessingError(err, categoryValkeyClientError, nil, "failed to get all properties")
		default:
			return nil, common.NewErrProcessingError(err, categoryValkeyClientError, nil, "failed to get all properties")
		}
	}

	result, err := resp.AsStrMap()
	if err != nil {
		return nil, common.NewErrProcessingError(err, categoryInternalError, nil, "unexpected hgetall response type for %s", clusterID)
	}

	ret := make([]entity.HostState, 0, len(result))

	for hostID, jsonHost := range result {
		model := State{}

		err := json.Unmarshal([]byte(jsonHost), &model)
		if err != nil {
			return nil, common.NewErrProcessingError(err, categoryInternalError, nil, "failed to unmarshal hgetall response for %s %s", clusterID, hostID)
		}

		hostState := mapToEntity(model)

		hostState.ClusterID = clusterID
		hostState.HostID = hostID

		ret = append(ret, hostState)
	}

	return ret, nil
}

func (r ValkeyRepo) isRetryable(err error) bool {
	// Network error
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// Valkey specfic error
	vErr, isValkeyError := valkey.IsValkeyErr(err)
	if !isValkeyError { // Retryable errors should have been handled before this block
		return false
	}

	return vErr.IsTryAgain()
}
