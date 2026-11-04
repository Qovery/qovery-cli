package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoDestroyKeepsQoveryConfigurationByDefault(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	output := &bytes.Buffer{}
	orchestrator := demoDestroyOrchestrator{local: local, out: output}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{ClusterName: "local-demo-user"})

	require.NoError(t, err)
	assert.True(t, local.clusterDeleted)
	assert.True(t, local.loopbackRemoved)
	assert.Contains(t, output.String(), "Operator association were preserved")
}

func TestDemoDestroyDeletesQoveryConfigurationWhenRequested(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	api := &fakeDemoDestroyAPI{clusterFound: true}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "local-demo-user",
		DeleteQoveryConfig: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "cluster-id", api.deletedClusterID)
}

func TestDemoDestroyKeepsLocalClusterWhenTheQoveryClusterIsRefused(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	api := &fakeDemoDestroyAPI{findErr: fmt.Errorf("cluster %q in this organization %w", "production", errNotADemoCluster)}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "production",
		DeleteQoveryConfig: true,
	})

	require.Error(t, err)
	assert.False(t, local.clusterDeleted)
	assert.False(t, local.loopbackRemoved)
	assert.Empty(t, api.deletedClusterID)
}

func TestDemoDestroyDeletesLocalResourcesWhenTheQoveryLookupFails(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	api := &fakeDemoDestroyAPI{findErr: errors.New("503 Service Unavailable")}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{OrganizationID: "organization-id", ClusterName: "local-demo-user", DeleteQoveryConfig: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run the command again")
	assert.True(t, local.clusterDeleted)
	assert.True(t, local.loopbackRemoved)
	assert.Empty(t, api.deletedClusterID)
}

func TestDemoDestroyDoesNotClaimARemoteDeletionWhenTheClusterIsMissing(t *testing.T) {
	var out bytes.Buffer
	api := &fakeDemoDestroyAPI{clusterFound: false}
	orchestrator := demoDestroyOrchestrator{api: api, local: &fakeDemoDestroyLocal{}, out: &out}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{OrganizationID: "organization-id", ClusterName: "local-demo-pierr", DeleteQoveryConfig: true})

	require.NoError(t, err)
	assert.Empty(t, api.deletedClusterID)
	assert.Contains(t, out.String(), "No Qovery cluster with this name exists in the current organization.")
	assert.NotContains(t, out.String(), "Qovery configuration were deleted")
}

func TestDemoDestroyDeletesTheQoveryClusterWhenTheLoopbackRemovalFails(t *testing.T) {
	var out bytes.Buffer
	api := &fakeDemoDestroyAPI{clusterFound: true}
	orchestrator := demoDestroyOrchestrator{api: api, local: &fakeDemoDestroyLocal{loopbackErr: errors.New("sudo: a password is required")}, out: &out}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{OrganizationID: "organization-id", ClusterName: "local-demo-user", DeleteQoveryConfig: true})

	require.NoError(t, err)
	assert.Equal(t, "cluster-id", api.deletedClusterID)
	assert.Contains(t, out.String(), "Warning: cannot remove the local loopback address")
}

func TestDemoDestroyDeletesRegistryWhenItIsNotShared(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo-user"}]`),
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{}}}}]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		nil,
	}}
	local := demoLocalCommands{runner: runner, goos: "linux"}

	err := local.DeleteK3dCluster(context.Background(), "local-demo-user")

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"k3d", "cluster", "list", "--output", "json"}}, runner.unlogged)
	assert.Equal(t, []string{"k3d", "cluster", "delete", "local-demo-user"}, runner.calls[3])
	assert.Equal(t, []string{"k3d", "registry", "delete", "k3d-" + demoRegistry}, runner.calls[5])
}

func TestDemoDestroyPreservesSharedRegistry(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo-user"}]`),
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{},"k3d-other-demo":{}}}}]`),
		nil,
	}}
	local := demoLocalCommands{runner: runner, goos: "linux"}

	err := local.DeleteK3dCluster(context.Background(), "local-demo-user")

	require.NoError(t, err)
	assert.Len(t, runner.calls, 4)
	assert.Equal(t, []string{"k3d", "cluster", "delete", "local-demo-user"}, runner.calls[3])
}

func TestDemoDestroyRemovesMacOSLoopbackAlias(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte("inet 172.42.0.3 netmask 0xffffffff"),
		nil,
	}}
	local := demoLocalCommands{runner: runner, goos: "darwin"}

	err := local.TeardownLoopback(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"sudo", "ifconfig", "lo0", "-alias", demoNodeIP}, runner.calls[1])
}

type fakeDemoDestroyAPI struct {
	clusterFound     bool
	findErr          error
	deletedClusterID string
}

func (f *fakeDemoDestroyAPI) FindCluster(context.Context, string, string) (string, bool, error) {
	return "cluster-id", f.clusterFound, f.findErr
}

func (f *fakeDemoDestroyAPI) DeleteClusterConfig(_ context.Context, _ string, clusterID string) error {
	f.deletedClusterID = clusterID
	return nil
}

type fakeDemoDestroyLocal struct {
	clusterDeleted  bool
	loopbackRemoved bool
	loopbackErr     error
}

func (f *fakeDemoDestroyLocal) CheckDestroyDependencies(context.Context) error { return nil }

func (f *fakeDemoDestroyLocal) DeleteK3dCluster(context.Context, string) error {
	f.clusterDeleted = true
	return nil
}

func (f *fakeDemoDestroyLocal) TeardownLoopback(context.Context) error {
	f.loopbackRemoved = f.loopbackErr == nil
	return f.loopbackErr
}
