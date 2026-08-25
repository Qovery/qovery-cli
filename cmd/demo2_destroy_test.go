package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemo2DestroyKeepsQoveryConfigurationByDefault(t *testing.T) {
	local := &fakeDemo2DestroyLocal{}
	output := &bytes.Buffer{}
	orchestrator := demo2DestroyOrchestrator{local: local, out: output}

	err := orchestrator.Destroy(context.Background(), demo2DestroyConfig{ClusterName: "local-demo2-user"})

	require.NoError(t, err)
	assert.True(t, local.clusterDeleted)
	assert.True(t, local.loopbackRemoved)
	assert.Contains(t, output.String(), "Operator association were preserved")
}

func TestDemo2DestroyDeletesQoveryConfigurationWhenRequested(t *testing.T) {
	local := &fakeDemo2DestroyLocal{}
	api := &fakeDemo2DestroyAPI{clusterFound: true}
	orchestrator := demo2DestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demo2DestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "local-demo2-user",
		DeleteQoveryConfig: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "cluster-id", api.deletedClusterID)
}

func TestDemo2DestroyDeletesRegistryWhenItIsNotShared(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo2-user"}]`),
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo2-user":{}}}}]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		nil,
	}}
	local := demo2LocalCommands{runner: runner, goos: "linux"}

	err := local.DeleteK3dCluster(context.Background(), "local-demo2-user")

	require.NoError(t, err)
	assert.Equal(t, []string{"k3d", "cluster", "delete", "local-demo2-user"}, runner.calls[3])
	assert.Equal(t, []string{"k3d", "registry", "delete", "k3d-" + demo2Registry}, runner.calls[5])
}

func TestDemo2DestroyPreservesSharedRegistry(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo2-user"}]`),
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo2-user":{},"k3d-other-demo":{}}}}]`),
		nil,
	}}
	local := demo2LocalCommands{runner: runner, goos: "linux"}

	err := local.DeleteK3dCluster(context.Background(), "local-demo2-user")

	require.NoError(t, err)
	assert.Len(t, runner.calls, 4)
	assert.Equal(t, []string{"k3d", "cluster", "delete", "local-demo2-user"}, runner.calls[3])
}

func TestDemo2DestroyRemovesMacOSLoopbackAlias(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte("inet 172.42.0.3 netmask 0xffffffff"),
		nil,
	}}
	local := demo2LocalCommands{runner: runner, goos: "darwin"}

	err := local.TeardownLoopback(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"sudo", "ifconfig", "lo0", "-alias", demo2NodeIP}, runner.calls[1])
}

type fakeDemo2DestroyAPI struct {
	clusterFound     bool
	deletedClusterID string
}

func (f *fakeDemo2DestroyAPI) FindCluster(context.Context, string, string) (string, bool, error) {
	return "cluster-id", f.clusterFound, nil
}

func (f *fakeDemo2DestroyAPI) DeleteClusterConfig(_ context.Context, _ string, clusterID string) error {
	f.deletedClusterID = clusterID
	return nil
}

type fakeDemo2DestroyLocal struct {
	clusterDeleted  bool
	loopbackRemoved bool
}

func (f *fakeDemo2DestroyLocal) CheckDestroyDependencies(context.Context) error { return nil }

func (f *fakeDemo2DestroyLocal) DeleteK3dCluster(context.Context, string) error {
	f.clusterDeleted = true
	return nil
}

func (f *fakeDemo2DestroyLocal) TeardownLoopback(context.Context) error {
	f.loopbackRemoved = true
	return nil
}
