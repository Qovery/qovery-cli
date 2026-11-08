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
