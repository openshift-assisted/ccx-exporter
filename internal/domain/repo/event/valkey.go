package event

import (
	"context"
	"errors"

	"github.com/valkey-io/valkey-go"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

type ValkeyRepo struct {
	client valkey.Client
}

func NewValkeyRepo(client valkey.Client) ValkeyRepo {
	return ValkeyRepo{client: client}
}

func (r ValkeyRepo) WriteEvent(ctx context.Context, event entity.Event) error {
	return errors.New("not implemented")
}

func (r ValkeyRepo) GetEvent(ctx context.Context, clusterID string) (entity.Event, error) {
	ret := entity.Event{}

	return ret, errors.New("not implemented")
}
