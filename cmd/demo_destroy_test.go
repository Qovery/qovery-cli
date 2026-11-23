package cmd

import (
	"bytes"
	"context"
	"encoding/json"
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
	clusterDeleted    bool
	loopbackRemoved   bool
	loopbackErr       error
	operatorRemoved   bool
	noOperatorRelease bool
	uninstallErr      error
}

func (f *fakeDemoDestroyLocal) UninstallOperator(context.Context) (bool, error) {
	if f.uninstallErr != nil {
		return false, f.uninstallErr
	}
	f.operatorRemoved = !f.noOperatorRelease
	return f.operatorRemoved, nil
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

func TestDemoDestroyWithAKubeContextKeepsTheKubeClusterAndQoveryConfiguration(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	output := &bytes.Buffer{}
	orchestrator := demoDestroyOrchestrator{api: &fakeDemoDestroyAPI{clusterFound: true}, local: local, out: output}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{ClusterName: "qls-a", KubeContext: "vcluster_qls-a"})

	require.NoError(t, err)
	assert.True(t, local.operatorRemoved)
	assert.False(t, local.clusterDeleted)
	assert.False(t, local.loopbackRemoved)
	assert.Contains(t, output.String(), "Qovery configuration of cluster \"qls-a\" was preserved")
}

func TestDemoDestroyWithAKubeContextDeletesTheQoveryCluster(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	api := &fakeDemoDestroyAPI{clusterFound: true}
	output := &bytes.Buffer{}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: output}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "qls-a",
		DeleteQoveryConfig: true,
		KubeContext:        "vcluster_qls-a",
	})

	require.NoError(t, err)
	assert.True(t, local.operatorRemoved)
	assert.False(t, local.clusterDeleted)
	assert.Equal(t, "cluster-id", api.deletedClusterID)
	assert.Contains(t, output.String(), "Kube context \"vcluster_qls-a\" was kept")
}

func TestDemoDestroyWithAKubeContextIsIdempotent(t *testing.T) {
	local := &fakeDemoDestroyLocal{noOperatorRelease: true}
	api := &fakeDemoDestroyAPI{}
	output := &bytes.Buffer{}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: output}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "qls-a",
		DeleteQoveryConfig: true,
		KubeContext:        "vcluster_qls-a",
	})

	require.NoError(t, err)
	assert.Empty(t, api.deletedClusterID)
	assert.Contains(t, output.String(), "No Qovery Operator release was found")
	assert.Contains(t, output.String(), "No Qovery cluster with this name exists")
	assert.NotContains(t, output.String(), "was removed")
}

func TestDemoDestroyWithAKubeContextKeepsTheQoveryClusterWhenTheUninstallFails(t *testing.T) {
	local := &fakeDemoDestroyLocal{uninstallErr: errors.New("helm uninstall failed with exit code 1")}
	api := &fakeDemoDestroyAPI{clusterFound: true}
	orchestrator := demoDestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
		OrganizationID:     "organization-id",
		ClusterName:        "qls-a",
		DeleteQoveryConfig: true,
		KubeContext:        "vcluster_qls-a",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot uninstall the Qovery Operator")
	assert.Empty(t, api.deletedClusterID)
}

func TestDemoUninstallOperatorRemovesOnlyOperatorReleases(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte(`[{"name":"qovery-operator","namespace":"qovery","chart":"qovery-operator-1.2.3"},{"name":"vcluster","namespace":"qls","chart":"vcluster-0.20.0"}]`),
		nil,
	}}
	local := newDemoLocalCommands(runner, "", "qls-a", "vcluster_qls-a", "/kube/qls-a.yaml")

	removed, err := local.UninstallOperator(context.Background())

	require.NoError(t, err)
	assert.True(t, removed)
	require.Len(t, runner.calls, 2)
	assert.Equal(t, []string{
		"helm", "list", "--kube-context", "vcluster_qls-a", "--kubeconfig", "/kube/qls-a.yaml", "--all-namespaces",
		"--deployed", "--failed", "--pending", "--superseded", "--uninstalling", "--max", "256", "--offset", "0", "--output", "json",
	}, runner.calls[0])
	assert.Equal(t, []string{
		"helm", "uninstall", "qovery-operator", "--namespace", "qovery",
		"--kube-context", "vcluster_qls-a", "--kubeconfig", "/kube/qls-a.yaml", "--wait", "--timeout", "10m",
	}, runner.calls[1])
}

func TestDemoDestroyWithAKubeContextOnlyNeedsHelm(t *testing.T) {
	runner := &lookPathRecordingDemoRunner{}
	local := newDemoLocalCommands(runner, "", "qls-a", "vcluster_qls-a", "")

	require.NoError(t, local.CheckDestroyDependencies(context.Background()))
	assert.Equal(t, []string{"helm"}, runner.lookedUp)
}

func TestDemoDestroyWithAKubeContextRefusesAClusterThatIsNotADemo(t *testing.T) {
	for _, deleteConfig := range []bool{false, true} {
		local := &fakeDemoDestroyLocal{}
		api := &fakeDemoDestroyAPI{findErr: fmt.Errorf("cluster %q in this organization %w", "production", errNotADemoCluster)}
		orchestrator := demoDestroyOrchestrator{api: api, local: local, out: &bytes.Buffer{}}

		err := orchestrator.Destroy(context.Background(), demoDestroyConfig{
			OrganizationID:     "organization-id",
			ClusterName:        "production",
			DeleteQoveryConfig: deleteConfig,
			KubeContext:        "production",
		})

		require.ErrorIs(t, err, errNotADemoCluster)
		assert.False(t, local.operatorRemoved)
		assert.Empty(t, api.deletedClusterID)
	}
}

func TestDemoDestroyWithAKubeContextDoesNotUninstallWhenTheLookupFails(t *testing.T) {
	local := &fakeDemoDestroyLocal{}
	orchestrator := demoDestroyOrchestrator{api: &fakeDemoDestroyAPI{findErr: errors.New("503 Service Unavailable")}, local: local, out: &bytes.Buffer{}}

	err := orchestrator.Destroy(context.Background(), demoDestroyConfig{ClusterName: "qls-a", KubeContext: "vcluster_qls-a"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the Qovery Operator was not uninstalled")
	assert.False(t, local.operatorRemoved)
}

func TestDemoUninstallOperatorPagesThroughEveryRelease(t *testing.T) {
	firstPage := make([]demoHelmRelease, demoHelmListPageSize)
	for i := range firstPage {
		firstPage[i] = demoHelmRelease{Name: fmt.Sprintf("app-%d", i), Namespace: "apps", Chart: "app-1.0.0"}
	}
	firstPageJSON, err := json.Marshal(firstPage)
	require.NoError(t, err)
	runner := &recordingDemoRunner{outputs: [][]byte{
		firstPageJSON,
		[]byte(`[{"name":"qovery-operator","namespace":"qovery","chart":"qovery-operator-1.2.3"}]`),
		nil,
	}}
	local := newDemoLocalCommands(runner, "", "qls-a", "vcluster_qls-a", "")

	removed, err := local.UninstallOperator(context.Background())

	require.NoError(t, err)
	assert.True(t, removed)
	require.Len(t, runner.calls, 3)
	assert.Equal(t, "0", runner.calls[0][indexOf(runner.calls[0], "--offset")+1])
	assert.Equal(t, "256", runner.calls[1][indexOf(runner.calls[1], "--offset")+1])
	assert.Equal(t, []string{"helm", "uninstall", "qovery-operator", "--namespace", "qovery"}, runner.calls[2][:5])
}

func TestDemoUpLegacyReleaseErrorFitsTheTarget(t *testing.T) {
	assert.Contains(t, legacyDemoReleaseError("").Error(), "destroy and recreate the local cluster")
	existing := legacyDemoReleaseError("vcluster_qls-a").Error()
	assert.Contains(t, existing, "helm uninstall qovery --namespace qovery --kube-context vcluster_qls-a")
	assert.NotContains(t, existing, "local cluster")
	assert.Contains(t, legacyDemoReleaseError("my ctx").Error(), "--kube-context 'my ctx'`")
}
