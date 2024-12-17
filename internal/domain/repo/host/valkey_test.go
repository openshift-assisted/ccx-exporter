package host_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
	"github.com/openshift-assisted/ccx-exporter/internal/domain/repo/host"
	"github.com/openshift-assisted/ccx-exporter/internal/factory"
	"github.com/openshift-assisted/ccx-exporter/pkg/pipeline"
)

// Helper

func startValkey(t *testing.T) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/sclorg/valkey-7-c10s:bf91acf0827dc5db216164aafe3d34beb245dcec",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections tcp"),
	}
	ret, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	testcontainers.CleanupContainer(t, ret)

	require.NoError(t, err, "failed to start valkey instance")

	return ret
}

func createValkeyClient(t *testing.T, container testcontainers.Container) valkey.Client {
	endpoint, err := container.Endpoint(context.Background(), "")
	require.NoError(t, err, "failed to get valkey endpoint")

	ret, err := factory.CreateValkeyClient(context.Background(), config.Valkey{URL: endpoint})
	require.NoError(t, err, "failed to create valkey client")

	return ret
}

// Test suite definition

type ValkeyDataIntegrationTestSuite struct {
	suite.Suite

	client    valkey.Client
	repo      host.ValkeyRepo
	container testcontainers.Container
}

func (s *ValkeyDataIntegrationTestSuite) SetupSuite() {
	t := s.T()

	s.container = startValkey(t)
	s.client = createValkeyClient(t, s.container)
	s.repo = host.NewValkeyRepo(s.client, time.Minute)
}

func (s *ValkeyDataIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	command := s.client.B().Flushall().Build()

	err := s.client.Do(ctx, command).Error()
	require.NoError(s.T(), err, "failed to clean valkey")
}

// Run test

func TestValkeyDataIntegrationTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ValkeyDataIntegrationTestSuite))
}

// Test

func (s *ValkeyDataIntegrationTestSuite) TestInsertAndRead() {
	ctx := context.Background()
	t := s.T()

	hostState := entity.HostState{ClusterID: "cluster-id", HostID: "host-id", Payload: map[string]interface{}{"test": "a"}}
	err := s.repo.WriteHostState(ctx, hostState)
	require.NoError(t, err, "failed to write host state")

	res, err := s.repo.GetHostStates(ctx, "cluster-id")
	require.NoError(t, err, "failed to get host states")

	require.Len(t, res, 1, "unexpected number of host state: %d", len(res))
	assert.Equal(t, hostState, res[0], "different host state")
}

func (s *ValkeyDataIntegrationTestSuite) TestOverwriteKey() {
	ctx := context.Background()
	t := s.T()

	hostState := entity.HostState{ClusterID: "cluster-id", HostID: "host-id", Payload: map[string]interface{}{"test": "a"}}
	err := s.repo.WriteHostState(ctx, hostState)
	require.NoError(t, err, "failed to write host state (1)")

	hostState.Payload = map[string]interface{}{"new": "data"}

	err = s.repo.WriteHostState(ctx, hostState)
	require.NoError(t, err, "failed to write host state (2)")

	res, err := s.repo.GetHostStates(ctx, "cluster-id")
	require.NoError(t, err, "failed to get host states")

	require.Len(t, res, 1, "unexpected number of host state: %d", len(res))
	assert.Equal(t, hostState, res[0], "different host state")
}

func (s *ValkeyDataIntegrationTestSuite) TestGetUnknowKey() {
	ctx := context.Background()
	t := s.T()

	res, err := s.repo.GetHostStates(ctx, "random")
	require.NoError(t, err, "failed to get host states")

	require.Len(t, res, 0, "unexpected number of host state: %d", len(res))
}

func (s *ValkeyDataIntegrationTestSuite) TestExpiration() {
	ctx := context.Background()
	t := s.T()

	hostState := entity.HostState{ClusterID: "cluster-id", HostID: "host-id", Payload: map[string]interface{}{"test": "a"}}
	err := s.repo.WriteHostState(ctx, hostState)
	require.NoError(t, err, "failed to write host state")

	// This is breaking black-box testing but is convenient...
	command := s.client.B().Ttl().Key("cluster-id").Build()

	resp := s.client.Do(ctx, command)
	require.NoError(t, resp.Error(), "failed to get TTL")

	ttl, err := resp.AsInt64() // ttl in second
	require.NoError(t, err, "TTL is not a int64")

	// This command returns -2 if key does not exist
	// This command returns -1 if key exists but has no TTL
	assert.Greater(t, ttl, int64(45), "ttl is supposed to be 1min") // Keeping some margin
}

func TestLosingConnection(t *testing.T) {
	t.Parallel()

	container := startValkey(t)
	client := createValkeyClient(t, container)
	repo := host.NewValkeyRepo(client, time.Minute)

	// stop the container
	err := container.Terminate(context.Background())
	require.NoError(t, err, "failed to terminate valkey")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = repo.GetHostStates(ctx, "unknown")
	require.Error(t, err, "get host states should fail")

	require.ErrorIs(t, err, pipeline.ErrRetryableError, "error should be retryable: %v", reflect.TypeOf(err))
}
