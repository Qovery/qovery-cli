package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoUpReportsTheErrorCodeOfTheFailedPhase(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		api         *fakeDemoAPI
		local       *fakeDemoLocal
		phase       string
		code        string
		safeMessage string
	}{
		{
			name:        "cluster name used by a cluster that is not a demo",
			api:         &fakeDemoAPI{findErr: errNotADemoCluster},
			local:       &fakeDemoLocal{},
			phase:       demoPhaseCredentialsAndCluster,
			code:        demoErrorClusterNameNotADemo,
			safeMessage: "A Qovery cluster with this name exists and is not a demo cluster.",
		},
		{
			name:        "Qovery API error",
			api:         &fakeDemoAPI{findErr: errors.New("502 Bad Gateway")},
			local:       &fakeDemoLocal{},
			phase:       demoPhaseCredentialsAndCluster,
			code:        demoErrorQoveryAPI,
			safeMessage: "A Qovery API request failed.",
		},
		{
			name:        "demo subnet already in use",
			api:         &fakeDemoAPI{},
			local:       &fakeDemoLocal{k3dErr: demoSubnetInUseError("k3d-local-demo-old")},
			phase:       demoPhaseLocalCluster,
			code:        demoErrorSubnetInUse,
			safeMessage: "Another Docker network already uses the demo subnet 172.42.0.0/16.",
		},
		{
			name:        "legacy release",
			api:         &fakeDemoAPI{},
			local:       &fakeDemoLocal{legacyRelease: true},
			phase:       demoPhaseLegacyReleaseCheck,
			code:        demoErrorLegacyRelease,
			safeMessage: "A legacy Qovery demo release exists on the local cluster.",
		},
		{
			name:        "operator install",
			api:         &fakeDemoAPI{},
			local:       &fakeDemoLocal{installErr: errors.New("helm failed")},
			phase:       demoPhaseOperatorBootstrap,
			code:        demoErrorOperatorInstall,
			safeMessage: "The Qovery Operator Helm installation failed.",
		},
		{
			name:        "operator heartbeat timeout",
			api:         &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{{Connected: false}}},
			local:       &fakeDemoLocal{},
			phase:       demoPhaseOperatorHeartbeat,
			code:        demoErrorOperatorHeartbeatTimeout,
			safeMessage: "The Qovery Operator did not report a fresh heartbeat in time.",
		},
		{
			name: "terminal deployment status",
			api: &fakeDemoAPI{
				operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
				clusterStatuses:  []string{"READY", "DEPLOYMENT_ERROR"},
			},
			local:       &fakeDemoLocal{},
			phase:       demoPhasePlatformCatalogDeployment,
			code:        demoErrorClusterDeployment,
			safeMessage: "The cluster deployment finished with status DEPLOYMENT_ERROR.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			progress := &recordingDemoProgress{}
			orchestrator := demoOrchestrator{
				api:      test.api,
				local:    test.local,
				clock:    &fakeDemoClock{now: now},
				out:      &bytes.Buffer{},
				progress: progress,
			}
			cfg := testDemoConfig()
			cfg.OperatorTimeout = 2 * time.Second
			cfg.PollInterval = time.Second

			err := orchestrator.Up(context.Background(), cfg)

			require.Error(t, err)
			last := progress.phases[len(progress.phases)-1]
			assert.Equal(t, demoPhaseExecution{
				Phase:            test.phase,
				Result:           demoPhaseResultFailed,
				ErrorCode:        test.code,
				SafeErrorMessage: test.safeMessage,
			}, demoPhaseExecution{
				Phase:            last.Phase,
				Result:           last.Result,
				ErrorCode:        last.ErrorCode,
				SafeErrorMessage: last.SafeErrorMessage,
			})
			code, safeMessage := demoFailureDetails(err)
			assert.Equal(t, test.code, code)
			assert.Equal(t, test.safeMessage, safeMessage)
		})
	}
}

func TestDemoFailureDetailsFallsBackToUnknownForUnclassifiedErrors(t *testing.T) {
	code, safeMessage := demoFailureDetails(errors.New("boom"))

	assert.Equal(t, demoUnknownErrorCode, code)
	assert.Equal(t, demoSafeErrorMessage, safeMessage)
}

func TestDemoCheckDependenciesListsEveryMissingCommandWithInstallHints(t *testing.T) {
	runner := &missingCommandsDemoRunner{missing: map[string]bool{"k3d": true, "helm": true}}
	local := &demoLocalCommands{runner: runner, goos: "darwin"}

	err := local.CheckDependencies(context.Background())

	require.Error(t, err)
	assert.Equal(t, "required commands are not installed: k3d, helm\n"+
		"Install them, then run `qovery demo up` again:\n"+
		"  brew install k3d helm\n"+
		"  k3d: https://k3d.io/stable/#installation\n"+
		"  helm: https://helm.sh/docs/intro/install/", err.Error())
	code, safeMessage := demoFailureDetails(err)
	assert.Equal(t, demoErrorMissingDependency, code)
	assert.Equal(t, "Required commands are not installed: k3d, helm.", safeMessage)
	assert.Empty(t, runner.calls)
}

func TestDemoCheckDependenciesOnlySuggestsBrewOnMacOS(t *testing.T) {
	runner := &missingCommandsDemoRunner{missing: map[string]bool{"kubectl": true}}
	local := &demoLocalCommands{runner: runner, goos: "linux"}

	err := local.CheckDependencies(context.Background())

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "brew")
	assert.Contains(t, err.Error(), "kubectl: https://kubernetes.io/docs/tasks/tools/")
}

func TestDemoCheckDependenciesDoesNotSuggestBrewForDocker(t *testing.T) {
	runner := &missingCommandsDemoRunner{missing: map[string]bool{"docker": true}}
	local := &demoLocalCommands{runner: runner, goos: "darwin"}

	err := local.CheckDependencies(context.Background())

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "brew")
	assert.Contains(t, err.Error(), "docker: https://docs.docker.com/get-started/get-docker/")
}

func TestDemoCheckDependenciesReportsDockerNotRunning(t *testing.T) {
	runner := &missingCommandsDemoRunner{}
	runner.errors = []error{errors.New("cannot connect to the Docker daemon")}
	local := &demoLocalCommands{runner: runner, goos: "darwin"}

	err := local.CheckDependencies(context.Background())

	require.Error(t, err)
	assert.Equal(t, [][]string{{"docker", "info"}}, runner.calls)
	code, safeMessage := demoFailureDetails(err)
	assert.Equal(t, demoErrorDockerNotRunning, code)
	assert.Equal(t, "Docker is not running.", safeMessage)
}

func TestDemoEnsureK3dClusterRefusesASubnetAlreadyInUse(t *testing.T) {
	tests := []struct {
		name    string
		network string
		message string
	}{
		{
			name:    "old demo cluster",
			network: `{"Name":"k3d-local-demo-old","IPAM":{"Config":[{"Subnet":"172.42.0.0/16"}]}}`,
			message: "the Docker network \"k3d-local-demo-old\" already uses the demo subnet 172.42.0.0/16\n" +
				"k3d created it for the cluster \"local-demo-old\". If it is an old demo cluster, delete it with `k3d cluster delete local-demo-old`. " +
				"If that cluster no longer exists, remove the network with `docker network rm k3d-local-demo-old`. Then run `qovery demo up` again.",
		},
		{
			name:    "other network with a wider range",
			network: `{"Name":"vpn","IPAM":{"Config":[{"Subnet":"172.40.0.0/13"}]}}`,
			message: "the Docker network \"vpn\" already uses the demo subnet 172.42.0.0/16\n" +
				"Remove this network with `docker network rm vpn`, then run `qovery demo up` again.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingDemoRunner{outputs: [][]byte{
				[]byte("[]"),
				[]byte("bridge-id\nother-id\n"),
				[]byte(`[{"Name":"bridge","IPAM":{"Config":[{"Subnet":"172.17.0.0/16"}]}},` + test.network + `]`),
			}}
			local := demoLocalCommands{runner: runner, goos: "darwin"}

			err := local.EnsureK3dCluster(context.Background(), "local-demo-user")

			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			code, _ := demoFailureDetails(err)
			assert.Equal(t, demoErrorSubnetInUse, code)
			assert.Len(t, runner.calls, 3)
		})
	}
}

func TestDemoEnsureK3dClusterCreatesTheClusterWhenTheSubnetCheckCannotDecide(t *testing.T) {
	demoNetwork := `{"Name":"k3d-local-demo-user","IPAM":{"Config":[{"Subnet":"172.42.0.0/16"}]}}`
	tests := []struct {
		name    string
		outputs [][]byte
		errors  []error
	}{
		{
			name:    "the cluster's own network is left over",
			outputs: [][]byte{[]byte("[]"), []byte("demo-id\n"), []byte(`[` + demoNetwork + `]`)},
		},
		{
			name:    "docker network ls fails",
			outputs: [][]byte{[]byte("[]"), nil},
			errors:  []error{nil, errors.New("exit status 1")},
		},
		{
			name:    "no network is listed",
			outputs: [][]byte{[]byte("[]"), []byte("")},
		},
		{
			name:    "docker network inspect fails",
			outputs: [][]byte{[]byte("[]"), []byte("gone-id\n"), nil},
			errors:  []error{nil, nil, errors.New("exit status 1")},
		},
		{
			name:    "docker network inspect prints something else than JSON",
			outputs: [][]byte{[]byte("[]"), []byte("bridge-id\n"), []byte("Emulate Docker CLI using podman.\n[]")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createIndex := len(test.outputs)
			runner := &recordingDemoRunner{
				outputs: append(test.outputs,
					nil,
					[]byte(`[{"name":"qovery-registry.lan"}]`),
					[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]`),
				),
				errors: test.errors,
			}
			local := demoLocalCommands{runner: runner, goos: "darwin"}

			err := local.EnsureK3dCluster(context.Background(), "local-demo-user")

			require.NoError(t, err)
			require.Greater(t, len(runner.calls), createIndex)
			assert.Equal(t, []string{"k3d", "cluster", "create", "local-demo-user"}, runner.calls[createIndex][:4])
		})
	}
}

type missingCommandsDemoRunner struct {
	recordingDemoRunner
	missing map[string]bool
}

func (r *missingCommandsDemoRunner) LookPath(name string) error {
	if r.missing[name] {
		return errors.New("executable file not found in $PATH")
	}
	return nil
}
