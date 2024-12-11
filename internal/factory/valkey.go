package factory

import (
	"context"
	"fmt"

	"github.com/openshift-assisted/ccx-exporter/internal/common"
	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/valkey-io/valkey-go"
)

func CreateValkeyClient(ctx context.Context, conf config.Valkey) (valkey.Client, common.CloseFunc, error) {
	ret, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{conf.URL},
		Password:    conf.Creds.Password,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	ping := ret.B().Ping().Build()

	err = ret.Do(ctx, ping).Error()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to ping valkey: %w", err)
	}

	shutdown := func(context.Context) error {
		ret.Close()

		return nil
	}

	return ret, shutdown, nil
}
