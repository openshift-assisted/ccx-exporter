package factory

import (
	"context"
	"fmt"

	"github.com/valkey-io/valkey-go"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
)

func CreateValkeyClient(ctx context.Context, conf config.Valkey) (valkey.Client, error) {
	ret, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{conf.URL},
		Password:    conf.Creds.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	ping := ret.B().Ping().Build()

	err = ret.Do(ctx, ping).Error()
	if err != nil {
		return nil, fmt.Errorf("failed to ping valkey: %w", err)
	}

	return ret, nil
}
