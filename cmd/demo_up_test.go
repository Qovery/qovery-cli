package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoUpFreshCreationOrder(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemoAPI{
		events:           &events,
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"READY", "DEPLOYED"},
	}
	local := &fakeDemoLocal{events: &events}
	orchestrator := demoOrchestrator{api: api, local: local, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.Equal(t, []string{
		"dependencies", "credentials", "find-cluster", "create-cluster", "k3d", "loopback",
		"legacy-release", "operator-config", "bootstrap", "attach", "install-operator", "operator-status",
		"cluster-status", "deploy", "cluster-status", "validate-workloads",
	}, events)
	assert.Equal(t, "ARM64", api.operatorCPUArchitecture)
}

func TestNewDemoAttemptUsesUUIDV7(t *testing.T) {
	attempt := newDemoAttempt()
	attemptID, err := uuid.Parse(attempt.id)

	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), attemptID.Version())
}

func TestDemoUpRerunReusesResources(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemoAPI{
		events:           &events,
		clusterFound:     true,
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"RESTARTED"},
	}
	local := &fakeDemoLocal{events: &events}
	orchestrator := demoOrchestrator{api: api, local: local, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.NotContains(t, events, "create-cluster")
	assert.Contains(t, events, "install-operator")
	assert.Contains(t, events, "deploy")
	assert.Equal(t, 1, api.ensureCredentialsCalls)
}

func TestDemoUpReportsEveryPhaseWithClusterCorrelation(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	progress := &recordingDemoProgress{}
	api := &fakeDemoAPI{
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"DEPLOYED"},
	}
	orchestrator := demoOrchestrator{
		api:      api,
		local:    &fakeDemoLocal{},
		clock:    &fakeDemoClock{now: now},
		out:      &bytes.Buffer{},
		progress: progress,
	}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.Equal(t, []string{"cluster-id"}, progress.resolvedClusterIDs)
	require.Len(t, progress.phases, 8)
	assert.Equal(t, []string{
		demoPhaseLocalDependencies,
		demoPhaseCredentialsAndCluster,
		demoPhaseLocalCluster,
		demoPhaseLegacyReleaseCheck,
		demoPhaseOperatorBootstrap,
		demoPhaseOperatorHeartbeat,
		demoPhasePlatformCatalogDeployment,
		demoPhaseWorkloadVerification,
	}, demoRecordedPhaseNames(progress.phases))
	for _, phase := range progress.phases {
		assert.Equal(t, demoPhaseResultSucceeded, phase.Result)
	}
	assert.Empty(t, progress.phases[0].ClusterID)
	for _, phase := range progress.phases[1:] {
		assert.Equal(t, "cluster-id", phase.ClusterID)
	}
}

func TestDemoUpReportsTheFailedPhaseAndStopsTheTimeline(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	progress := &recordingDemoProgress{}
	orchestrator := demoOrchestrator{
		api:      &fakeDemoAPI{},
		local:    &fakeDemoLocal{legacyRelease: true},
		clock:    &fakeDemoClock{now: now},
		out:      &bytes.Buffer{},
		progress: progress,
	}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	require.Len(t, progress.phases, 4)
	assert.Equal(t, demoPhaseLegacyReleaseCheck, progress.phases[3].Phase)
	assert.Equal(t, demoPhaseResultFailed, progress.phases[3].Result)
	assert.Equal(t, "cluster-id", progress.phases[3].ClusterID)
}

func TestDemoClusterRequestSerializesIsDemo(t *testing.T) {
	request := newDemoClusterRequest("local-demo-user", demoCredential{ID: "credential-id", Name: "on-premise"})

	payload, err := json.Marshal(request)

	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, true, decoded["is_demo"])
	assert.Equal(t, "ON_PREMISE", decoded["cloud_provider"])
	assert.Equal(t, "SELF_MANAGED", decoded["kubernetes"])
	assert.Equal(t, false, decoded["production"])
}

func TestDemoOperatorConfigurationPreservesExistingConfiguration(t *testing.T) {
	configuration := testDemoPlatformConfiguration(demoPlatformTemplateKey, "0.1.0")
	configuration.Platform.SetLayerSelections(map[string]bool{"qovery-stack": false})
	configuration.Platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {"existing": "value"},
	})
	configuration.ClusterInputs = map[string]map[string]string{"qovery-operator": {"input": "value"}}

	request, err := newDemoOperatorConfigurationRequest(configuration, "ARM64")

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"qovery-stack": false}, request.Platform.GetLayerSelections())
	assert.Equal(t, map[string]map[string]interface{}{
		"qovery-operator": {"existing": "value", "cpuArchitectures": "ARM64"},
	}, request.Platform.GetManagedConfig())
	assert.Equal(t, map[string]map[string]string{"qovery-operator": {"input": "value"}}, request.ClusterInputs)
}

func TestDemoOperatorConfigurationRequestAlwaysSendsClusterInputs(t *testing.T) {
	configuration := &qovery.ClusterPlatformConfigurationResponse{
		Platform: *qovery.NewPlatformSelection(demoPlatformTemplateKey, "0.1.0"),
	}

	request, err := newDemoOperatorConfigurationRequest(configuration, "AMD64")

	require.NoError(t, err)
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.1.0",
			"layerSelections": {},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "AMD64"}}
		},
		"clusterInputs": {}
	}`, string(payload))
}

func TestDemoPlatformConfigurationSelectsTheDemoTemplate(t *testing.T) {
	catalog := qovery.NewPlatformTemplateCatalogResponse([]qovery.PlatformTemplateSummaryResponse{
		{
			Key:     "qovery-cluster-v0",
			Version: "0.1.0",
		},
		{
			Key:     demoPlatformTemplateKey,
			Version: "0.2.0",
			Layers: []qovery.PlatformTemplateLayerResponse{
				{Key: "qovery-stack", Mandatory: false, EnabledByDefault: true},
				{Key: "dns-certificates", Mandatory: true, EnabledByDefault: true},
			},
		},
	})

	configuration, err := newDemoPlatformConfiguration(catalog)

	require.NoError(t, err)
	assert.Equal(t, demoPlatformTemplateKey, configuration.Platform.TemplateKey)
	assert.Equal(t, "0.2.0", configuration.Platform.TemplateVersion)
	assert.Equal(t, map[string]bool{"qovery-stack": true}, configuration.Platform.GetLayerSelections())
	assert.Empty(t, configuration.Platform.GetManagedConfig())
	assert.Empty(t, configuration.ClusterInputs)
}

func TestDemoPlatformConfigurationRequiresTheDemoTemplate(t *testing.T) {
	catalog := qovery.NewPlatformTemplateCatalogResponse([]qovery.PlatformTemplateSummaryResponse{{
		Key:     "qovery-cluster-v0",
		Version: "0.1.0",
	}})

	_, err := newDemoPlatformConfiguration(catalog)

	require.EqualError(t, err, `qovery returned no "qovery-demo-v0" platform template for the local demo`)
}

func TestDemoPlatformCatalogFixtureExercisesFieldSchemaVariants(t *testing.T) {
	var catalog qovery.PlatformTemplateCatalogResponse
	require.NoError(t, json.Unmarshal([]byte(testDemoPlatformCatalog), &catalog))
	require.Len(t, catalog.Results, 2)
	demo := catalog.Results[1]
	bootstrap := demo.GetBootstrapComponent()
	require.Len(t, bootstrap.Fields, 1)
	fields := demo.Layers[0].Components[0].Fields
	require.Len(t, fields, 2)

	assert.Equal(t, demoOperatorComponentKey, bootstrap.Key)
	assert.NotNil(t, bootstrap.Fields[0].ScalarFieldSchemaResponse)
	assert.NotNil(t, fields[0].ScalarFieldSchemaResponse)
	require.NotNil(t, fields[1].ArrayFieldSchemaResponse)
	require.NotNil(t, fields[1].ArrayFieldSchemaResponse.Items.ObjectArrayItemResponse)
	assert.Len(t, fields[1].ArrayFieldSchemaResponse.Items.ObjectArrayItemResponse.Fields, 2)
}

func TestSelectDemoPlatformConfigurationPreservesMatchingConfiguration(t *testing.T) {
	existing := testDemoPlatformConfiguration(demoPlatformTemplateKey, "0.1.0")
	existing.Platform.SetManagedConfig(map[string]map[string]interface{}{"qovery-operator": {"existing": "value"}})
	desired := testDemoPlatformConfiguration(demoPlatformTemplateKey, "0.1.0")

	selected := selectDemoPlatformConfiguration(existing, desired)

	assert.Same(t, existing, selected)
}

func TestSelectDemoPlatformConfigurationReplacesStandardConfiguration(t *testing.T) {
	existing := testDemoPlatformConfiguration("qovery-cluster-v0", "0.1.0")
	existing.Platform.SetLayerSelections(map[string]bool{"log-infra": false})
	desired := testDemoPlatformConfiguration(demoPlatformTemplateKey, "0.1.0")
	desired.Platform.SetLayerSelections(map[string]bool{})

	selected := selectDemoPlatformConfiguration(existing, desired)

	assert.Same(t, desired, selected)
	assert.NotContains(t, selected.Platform.GetLayerSelections(), "log-infra")
}

func TestDemoPlatformConfigurationRejectsRedactedValuesItWouldSendBack(t *testing.T) {
	configuration := testDemoPlatformConfiguration(demoPlatformTemplateKey, "0.1.0")
	configuration.Platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {"cpuArchitectures": "<redacted>"},
		"karpenter":       {"replicas": 2, "token": "<redacted>"},
		"cert-manager":    {"email": "<redacted>", "issuer": "<redacted>"},
		"external-dns":    {"provider": "cloudflare"},
	})
	request, err := newDemoOperatorConfigurationRequest(configuration, "ARM64")
	require.NoError(t, err)

	err = ensureDemoRequestHasNoRedactedValues(request)

	require.EqualError(t, err, "qovery returned redacted values for cert-manager.email, cert-manager.issuer, karpenter.token; qovery demo sends the whole configuration back and would store the redaction marker instead, so nothing was saved. Retry later; if it persists, these fields are sensitive and qovery demo cannot keep them: clear them, or recreate the cluster with `qovery demo destroy --delete-qovery-config`")
}

func TestNormalizeDemoCPUArchitecture(t *testing.T) {
	arm64, err := normalizeDemoCPUArchitecture(" arm64 ")
	require.NoError(t, err)
	amd64, err := normalizeDemoCPUArchitecture("AMD64")
	require.NoError(t, err)
	_, err = normalizeDemoCPUArchitecture("386")

	assert.Equal(t, "ARM64", arm64)
	assert.Equal(t, "AMD64", amd64)
	require.EqualError(t, err, `unsupported local CPU architecture "386"`)
}

func TestDemoPlatformConfigurationAcceptsUnredactedValues(t *testing.T) {
	platform := qovery.NewPlatformSelection(demoPlatformTemplateKey, "0.1.0")
	platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {
			"cpuArchitectures": "ARM64",
			"note":             "redacted",
			"nested":           map[string]interface{}{"token": "<redacted>"},
		},
	})
	request := qovery.NewClusterPlatformConfigurationRequest(*platform, map[string]map[string]string{})

	require.NoError(t, ensureDemoRequestHasNoRedactedValues(*request))
}

func TestDemoConfigureOperatorCreatesTheDemoConfiguration(t *testing.T) {
	server := newFakeDemoPlatformServer(t, "")

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"GET /organization/organization-id/platformTemplate",
		"GET /v1/cluster/cluster-id/platformConfiguration",
		"PUT /v1/cluster/cluster-id/platformConfiguration",
	}, server.requests())
	assert.JSONEq(t, testDemoDefaultPlatformConfigurationRequest, server.savedConfiguration())
}

func TestDemoConfigureOperatorReplacesAnotherRedactedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name            string
		templateKey     string
		templateVersion string
	}{
		{name: "another template", templateKey: "qovery-cluster-v0", templateVersion: "0.2.0"},
		{name: "another version", templateKey: demoPlatformTemplateKey, templateVersion: "0.1.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeDemoPlatformServer(t, fmt.Sprintf(`{
				"clusterId": "cluster-id",
				"organizationId": "organization-id",
				"platform": {
					"templateKey": %q,
					"templateVersion": %q,
					"managedConfig": {"qovery-operator": {"cpuArchitectures": "<redacted>", "logLevel": "<redacted>"}}
				},
				"clusterInputs": {"qovery-operator": {"input": "value"}},
				"layers": []
			}`, test.templateKey, test.templateVersion))

			err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

			require.NoError(t, err)
			assert.JSONEq(t, testDemoDefaultPlatformConfigurationRequest, server.savedConfiguration())
		})
	}
}

func TestDemoConfigureOperatorOverwritesARedactedCPUArchitecture(t *testing.T) {
	server := newFakeDemoPlatformServer(t, `{
		"clusterId": "cluster-id",
		"organizationId": "organization-id",
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": false},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "<redacted>"}}
		},
		"clusterInputs": {"qovery-operator": {"input": "value"}},
		"layers": []
	}`)

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": false},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}
		},
		"clusterInputs": {"qovery-operator": {"input": "value"}}
	}`, server.savedConfiguration())
}

func TestDemoConfigureOperatorUpdatesTheExistingDemoConfiguration(t *testing.T) {
	server := newFakeDemoPlatformServer(t, `{
		"clusterId": "cluster-id",
		"organizationId": "organization-id",
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": false},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "AMD64", "logLevel": "debug"}}
		},
		"clusterInputs": {"qovery-operator": {"input": "value"}},
		"layers": []
	}`)

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": false},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "logLevel": "debug"}}
		},
		"clusterInputs": {"qovery-operator": {"input": "value"}}
	}`, server.savedConfiguration())
}

func TestDemoConfigureOperatorDoesNotSaveRedactedConfiguration(t *testing.T) {
	server := newFakeDemoPlatformServer(t, `{
		"clusterId": "cluster-id",
		"organizationId": "organization-id",
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": true},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "<redacted>", "logLevel": "<redacted>"}}
		},
		"clusterInputs": {},
		"layers": []
	}`)

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.ErrorContains(t, err, "qovery returned redacted values for qovery-operator.logLevel;")
	assert.Equal(t, []string{
		"GET /organization/organization-id/platformTemplate",
		"GET /v1/cluster/cluster-id/platformConfiguration",
	}, server.requests())
	assert.Empty(t, server.savedConfiguration())
}

func TestDemoConfigureOperatorSkipsAnUpToDateConfiguration(t *testing.T) {
	server := newFakeDemoPlatformServer(t, `{
		"clusterId": "cluster-id",
		"organizationId": "organization-id",
		"platform": {
			"templateKey": "qovery-demo-v0",
			"templateVersion": "0.2.0",
			"layerSelections": {"qovery-stack": true},
			"managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "logLevel": "<redacted>"}}
		},
		"clusterInputs": {},
		"layers": []
	}`)

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"GET /organization/organization-id/platformTemplate",
		"GET /v1/cluster/cluster-id/platformConfiguration",
	}, server.requests())
	assert.Empty(t, server.savedConfiguration())
}

func TestDemoConfigureOperatorRejectsAnUnsupportedArchitectureBeforeCallingQovery(t *testing.T) {
	server := newFakeDemoPlatformServer(t, "")

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "386")

	require.EqualError(t, err, `unsupported local CPU architecture "386"`)
	assert.Empty(t, server.requests())
}

func TestDemoConfigureOperatorReturnsQoveryErrors(t *testing.T) {
	const (
		listTemplates     = "GET /organization/organization-id/platformTemplate"
		readConfiguration = "GET /v1/cluster/cluster-id/platformConfiguration"
		saveConfiguration = "PUT /v1/cluster/cluster-id/platformConfiguration"
		conflict          = `{"type":"about:blank","title":"Conflict","status":409,"detail":"The cluster uses a platform profile owned by another cluster"}`
	)
	for _, test := range []struct {
		name     string
		call     string
		status   int
		body     string
		error    string
		requests []string
	}{
		{
			name:     "catalog unavailable",
			call:     listTemplates,
			status:   http.StatusServiceUnavailable,
			error:    "cannot list the platform templates: 503 Service Unavailable",
			requests: []string{listTemplates},
		},
		{
			name:     "configuration forbidden",
			call:     readConfiguration,
			status:   http.StatusForbidden,
			error:    "cannot read the cluster platform configuration: 403 Forbidden",
			requests: []string{listTemplates, readConfiguration},
		},
		{
			name:     "configuration unreadable",
			call:     readConfiguration,
			status:   http.StatusInternalServerError,
			error:    "cannot read the cluster platform configuration: 500 Internal Server Error",
			requests: []string{listTemplates, readConfiguration},
		},
		{
			name:     "profile not editable",
			call:     saveConfiguration,
			status:   http.StatusConflict,
			body:     conflict + "\n",
			error:    "cannot save the cluster platform configuration: 409 Conflict: " + conflict,
			requests: []string{listTemplates, readConfiguration, saveConfiguration},
		},
		{
			name:     "configuration rejected",
			call:     saveConfiguration,
			status:   http.StatusBadRequest,
			error:    "cannot save the cluster platform configuration: 400 Bad Request",
			requests: []string{listTemplates, readConfiguration, saveConfiguration},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeDemoPlatformServer(t, "")
			server.fail(test.call, test.status, test.body)

			err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

			require.EqualError(t, err, test.error)
			assert.Equal(t, test.requests, server.requests())
		})
	}
}

func TestDemoUpRejectsLegacyQoveryRelease(t *testing.T) {
	events := []string{}
	api := &fakeDemoAPI{events: &events}
	local := &fakeDemoLocal{events: &events, legacyRelease: true}
	orchestrator := demoOrchestrator{api: api, local: local, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "adopting an old demo is not supported")
	assert.NotContains(t, events, "bootstrap")
	assert.NotContains(t, events, "install-operator")
}

func TestBuildDemoHelmArgsUsesStructuredBootstrap(t *testing.T) {
	bootstrap := testDemoBootstrap()

	args, err := buildDemoHelmArgs(bootstrap, "/tmp/protected-values.yaml", "k3d-local-demo-user")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"upgrade", "--install", "--kube-context", "k3d-local-demo-user", "qovery-operator", "oci://registry.example/qovery-operator",
		"--version", "1.2.3", "--namespace", "qovery", "--values", "/tmp/protected-values.yaml",
		"--create-namespace", "--atomic", "--wait", "--timeout", "15m",
	}, args)
	assert.NotContains(t, strings.Join(args, " "), "ignored helm command")
}

func TestBuildDemoHelmArgsRejectsUmbrellaChart(t *testing.T) {
	bootstrap := testDemoBootstrap()
	bootstrap.ChartReference = "oci://public.ecr.aws/example/charts/qovery"

	_, err := buildDemoHelmArgs(bootstrap, "/tmp/protected-values.yaml", "k3d-local-demo-user")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "umbrella chart")
}

func TestDemoOperatorValuesFileIsProtectedAndRemoved(t *testing.T) {
	runner := &inspectingDemoRunner{t: t}
	local := demoLocalCommands{runner: runner, goos: "linux", tempDir: t.TempDir()}

	err := local.InstallOperator(context.Background(), testDemoBootstrap())

	require.NoError(t, err)
	require.NotEmpty(t, runner.valuesPath)
	_, statErr := os.Stat(runner.valuesPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDemoExecRunnerKeepsCommandOutputInTheDebugLog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demoExecRunner{out: &terminal, log: &log}

	_, err := runner.RunQuiet(context.Background(), "/bin/sh", "-c", "printf diagnostic >&2; exit 7")

	require.Error(t, err)
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "$ /bin/sh -c")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemoExecRunnerStreamsSuccessfulCommandsInDebugMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demoExecRunner{out: &terminal, log: &log, debug: true}

	_, err := runner.RunQuiet(context.Background(), "/bin/sh", "-c", "printf diagnostic")

	require.NoError(t, err)
	assert.Contains(t, terminal.String(), "$ /bin/sh -c")
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemoExecRunnerKeepsUnloggedOutputOutOfTheDebugLog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demoExecRunner{out: &terminal, log: &log, debug: true}

	output, err := runner.RunUnlogged(context.Background(), "/bin/sh", "-c", "printf 'K3S_%s=%s' TOKEN ok42")
	require.NoError(t, err)
	_, err = runner.RunUnlogged(context.Background(), "/bin/sh", "-c", "printf 'K3S_%s=%s' TOKEN ko42 >&2; exit 7")
	require.Error(t, err)

	assert.Equal(t, "K3S_TOKEN=ok42", string(output))
	assert.Contains(t, log.String(), "$ /bin/sh -c printf 'K3S_%s=%s' TOKEN ok42\n")
	assert.NotContains(t, log.String(), "K3S_TOKEN=")
	assert.NotContains(t, terminal.String(), "K3S_TOKEN=ok42")
	assert.Contains(t, terminal.String(), "K3S_TOKEN=ko42")
}

func TestDemoExecRunnerInterruptsCommandsOnCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner := demoExecRunner{}
	time.AfterFunc(200*time.Millisecond, cancel)

	output, _ := runner.RunQuiet(ctx, "/bin/sh", "-c", "trap 'echo interrupted; exit 0' INT; sleep 5 >/dev/null 2>&1 & wait")

	assert.Equal(t, "interrupted\n", string(output))
}

func TestDemoExecRunnerStreamsActionCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demoExecRunner{out: &terminal, log: &log}

	_, err := runner.Run(context.Background(), "/bin/sh", "-c", "printf diagnostic")

	require.NoError(t, err)
	assert.Contains(t, terminal.String(), "$ /bin/sh -c")
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemoUpTimesOutWaitingForOperator(t *testing.T) {
	clock := &fakeDemoClock{now: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)}
	api := &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{{Connected: false}}}
	local := &fakeDemoLocal{}
	orchestrator := demoOrchestrator{api: api, local: local, clock: clock, out: &bytes.Buffer{}}
	cfg := testDemoConfig()
	cfg.OperatorTimeout = 2 * time.Second
	cfg.PollInterval = time.Second

	err := orchestrator.Up(context.Background(), cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Equal(t, 0, api.deployCalls)
}

func TestDemoUpDoesNotDeployBeforeHeartbeat(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemoAPI{
		events: &events,
		operatorStatuses: []demoOperatorStatus{
			{Connected: true},
			readyDemoOperatorStatus(now.Add(time.Second)),
		},
		clusterStatuses: []string{"DEPLOYED"},
	}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{events: &events}, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}
	cfg := testDemoConfig()
	cfg.PollInterval = time.Second

	err := orchestrator.Up(context.Background(), cfg)

	require.NoError(t, err)
	firstStatus := indexOf(events, "operator-status")
	secondStatus := indexOf(events[firstStatus+1:], "operator-status") + firstStatus + 1
	deploy := indexOf(events, "deploy")
	assert.Greater(t, deploy, secondStatus)
}

func TestDemoUpDeploymentWaitsForASuccessfulStatus(t *testing.T) {
	now := time.Now()
	api := &fakeDemoAPI{
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"READY", "READY", "DEPLOYING", "DEPLOYED"},
	}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.Equal(t, 4, api.clusterStatusCalls)
}

func TestDemoUpDeploysOnceTheDeploymentInProgressSettles(t *testing.T) {
	now := time.Now()
	events := []string{}
	api := &fakeDemoAPI{
		events:           &events,
		clusterFound:     true,
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"DEPLOYING", "DEPLOYING", "DEPLOYMENT_ERROR", "DEPLOYED"},
	}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.Equal(t, []string{"cluster-status", "cluster-status", "cluster-status", "deploy", "cluster-status"}, events[indexOf(events, "cluster-status"):])
}

func TestIsDemoDeploymentInProgress(t *testing.T) {
	inProgress := []string{"BUILDING", "CANCELING", "DELETE_QUEUED", "DELETING", "DEPLOYING", "DEPLOYMENT_QUEUED", "QUEUED", "STOPPING", "STOP_QUEUED", "RESTART_QUEUED", "RESTARTING"}
	settled := []string{"READY", "DEPLOYED", "RESTARTED", "STOPPED", "DELETED", "CANCELED", "BUILD_ERROR", "DEPLOYMENT_ERROR", "DELETE_ERROR", "STOP_ERROR", "RESTART_ERROR", "INVALID_CREDENTIALS"}
	for _, status := range inProgress {
		assert.True(t, isDemoDeploymentInProgress(status), status)
	}
	for _, status := range settled {
		assert.False(t, isDemoDeploymentInProgress(status), status)
	}
}

func TestDemoUpReturnsTerminalDeploymentError(t *testing.T) {
	now := time.Now()
	api := &fakeDemoAPI{
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"READY", "DEPLOYING", "DEPLOYMENT_ERROR"},
	}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: &fakeDemoClock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEPLOYMENT_ERROR")
}

func TestDemoUpRedactsValuesFromErrors(t *testing.T) {
	secret := "super-secret-cluster-jwt"
	api := &fakeDemoAPI{bootstrap: demoBootstrap{
		ReleaseName: "qovery-operator", ChartReference: "chart", ChartVersion: "1", Namespace: "qovery", ValuesYAML: "token: " + secret,
	}}
	local := &fakeDemoLocal{installErr: errors.New("helm failed with values_yaml token: " + secret)}
	orchestrator := demoOrchestrator{api: api, local: local, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, err.Error(), "values_yaml")
	assert.Contains(t, err.Error(), "redacted")
}

func TestDemoEnsureK3dClusterUsesPinnedSubstrate(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte("[]"),
		nil,
		[]byte(`[{"name":"qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]`),
	}}
	local := demoLocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo-user")

	require.NoError(t, err)
	require.Len(t, runner.calls, 4)
	assert.Equal(t, [][]string{{"k3d", "cluster", "list", "--output", "json"}}, runner.unlogged)
	assert.Equal(t, "k3d", runner.calls[1][0])
	joined := strings.Join(runner.calls[1][1:], " ")
	assert.Contains(t, joined, demoK3sImage)
	assert.Contains(t, joined, demoSubnet)
	assert.Contains(t, joined, "--node-ip="+demoNodeIP+"@server:0")
	assert.Contains(t, joined, "--disable=traefik@server:*")
	assert.Contains(t, joined, demoRegistry)
	assert.Contains(t, joined, "80:80@loadbalancer")
	assert.Contains(t, joined, "443:443@loadbalancer")
}

func TestDemoEnsureK3dClusterStartsExistingCluster(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo-user"}]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]`),
	}}
	local := demoLocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo-user")

	require.NoError(t, err)
	assert.Equal(t, []string{"k3d", "cluster", "start", "local-demo-user"}, runner.calls[1])
	assert.Equal(t, []string{"k3d", "registry", "list", "--output", "json"}, runner.calls[2])
}

func TestDemoEnsureK3dClusterRecreatesMissingRegistry(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo-user"}]`),
		nil,
		[]byte(`[]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["k3d-qovery-registry.lan"]}}}}]`),
		nil,
		nil,
	}}
	local := demoLocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo-user")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"k3d", "registry", "create", demoRegistry,
		"--default-network", "k3d-local-demo-user",
		"--no-help",
	}, runner.calls[3])
	assert.Equal(t, []string{
		"docker", "network", "connect", "--alias", demoRegistry,
		"k3d-local-demo-user", "k3d-" + demoRegistry,
	}, runner.calls[7])
}

func TestDemoValidateWorkloadsRequiresOperatorAndRejectsPermanentEngine(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		runner := &recordingDemoRunner{outputs: [][]byte{[]byte(`{"items":[{"metadata":{"name":"qovery-operator"}}]}`)}}
		local := demoLocalCommands{runner: runner, goos: "linux", kubeContext: "k3d-local-demo-user"}
		require.NoError(t, local.ValidateWorkloads(context.Background(), "qovery"))
		assert.Equal(t, []string{"kubectl", "--context", "k3d-local-demo-user", "--namespace", "qovery", "get", "deployments", "--output", "json"}, runner.calls[0])
	})

	t.Run("permanent engine", func(t *testing.T) {
		runner := &recordingDemoRunner{outputs: [][]byte{[]byte(`{"items":[{"metadata":{"name":"qovery-operator"}},{"metadata":{"name":"qovery-engine"}}]}`)}}
		local := demoLocalCommands{runner: runner, goos: "linux"}
		err := local.ValidateWorkloads(context.Background(), "qovery")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected permanent Deployment qovery-engine")
	})
}

func testDemoConfig() demoConfig {
	return demoConfig{
		OrganizationID:    "organization-id",
		ClusterName:       "local-demo-user",
		CPUArchitecture:   "ARM64",
		OperatorTimeout:   time.Minute,
		DeploymentTimeout: time.Minute,
		PollInterval:      time.Second,
	}
}

func testDemoBootstrap() demoBootstrap {
	return demoBootstrap{
		ReleaseName:    "qovery-operator",
		ChartReference: "oci://registry.example/qovery-operator",
		ChartVersion:   "1.2.3",
		Namespace:      "qovery",
		ValuesYAML:     "secretToken: highly-sensitive\n",
	}
}

func readyDemoOperatorStatus(now time.Time) demoOperatorStatus {
	heartbeat := now
	return demoOperatorStatus{Connected: true, LastHeartbeat: &heartbeat}
}

type fakeDemoAPI struct {
	events                  *[]string
	clusterFound            bool
	findErr                 error
	bootstrap               demoBootstrap
	operatorStatuses        []demoOperatorStatus
	clusterStatuses         []string
	ensureCredentialsCalls  int
	operatorStatusCalls     int
	clusterStatusCalls      int
	deployCalls             int
	deployStatus            string
	operatorCPUArchitecture string
}

func (f *fakeDemoAPI) event(value string) {
	if f.events != nil {
		*f.events = append(*f.events, value)
	}
}

func (f *fakeDemoAPI) EnsureOnPremiseCredentials(context.Context, string) (demoCredential, error) {
	f.event("credentials")
	f.ensureCredentialsCalls++
	return demoCredential{ID: "credential-id", Name: "on-premise"}, nil
}

func (f *fakeDemoAPI) FindCluster(context.Context, string, string) (string, bool, error) {
	f.event("find-cluster")
	return "cluster-id", f.clusterFound, f.findErr
}

func (f *fakeDemoAPI) CreateCluster(context.Context, string, string, demoCredential) (string, error) {
	f.event("create-cluster")
	return "cluster-id", nil
}

func (f *fakeDemoAPI) ConfigureOperator(_ context.Context, _ string, _ string, cpuArchitecture string) error {
	f.event("operator-config")
	f.operatorCPUArchitecture = cpuArchitecture
	return nil
}

func (f *fakeDemoAPI) GetOperatorBootstrap(context.Context, string, string) (demoBootstrap, error) {
	f.event("bootstrap")
	if f.bootstrap.ReleaseName != "" {
		return f.bootstrap, nil
	}
	return testDemoBootstrap(), nil
}

func (f *fakeDemoAPI) AttachOperator(context.Context, string, string) error {
	f.event("attach")
	return nil
}

func (f *fakeDemoAPI) GetOperatorStatus(context.Context, string, string) (demoOperatorStatus, error) {
	f.event("operator-status")
	index := f.operatorStatusCalls
	f.operatorStatusCalls++
	if len(f.operatorStatuses) == 0 {
		return demoOperatorStatus{}, nil
	}
	if index >= len(f.operatorStatuses) {
		index = len(f.operatorStatuses) - 1
	}
	return f.operatorStatuses[index], nil
}

func (f *fakeDemoAPI) DeployCluster(context.Context, string, string) (string, error) {
	f.event("deploy")
	f.deployCalls++
	if f.deployStatus == "" {
		return "DEPLOYMENT_QUEUED", nil
	}
	return f.deployStatus, nil
}

func (f *fakeDemoAPI) GetClusterStatus(context.Context, string, string) (string, error) {
	f.event("cluster-status")
	index := f.clusterStatusCalls
	f.clusterStatusCalls++
	if len(f.clusterStatuses) == 0 {
		return "DEPLOYED", nil
	}
	if index >= len(f.clusterStatuses) {
		index = len(f.clusterStatuses) - 1
	}
	return f.clusterStatuses[index], nil
}

type fakeDemoLocal struct {
	events        *[]string
	legacyRelease bool
	installErr    error
}

func (f *fakeDemoLocal) event(value string) {
	if f.events != nil {
		*f.events = append(*f.events, value)
	}
}

func (f *fakeDemoLocal) CheckDependencies(context.Context) error {
	f.event("dependencies")
	return nil
}

func (f *fakeDemoLocal) EnsureK3dCluster(context.Context, string) error {
	f.event("k3d")
	return nil
}

func (f *fakeDemoLocal) EnsureLoopback(context.Context) error {
	f.event("loopback")
	return nil
}

func (f *fakeDemoLocal) LegacyQoveryReleaseExists(context.Context) (bool, error) {
	f.event("legacy-release")
	return f.legacyRelease, nil
}

func (f *fakeDemoLocal) InstallOperator(context.Context, demoBootstrap) error {
	f.event("install-operator")
	return f.installErr
}

func (f *fakeDemoLocal) ValidateWorkloads(context.Context, string) error {
	f.event("validate-workloads")
	return nil
}

type fakeDemoClock struct {
	now time.Time
}

func (f *fakeDemoClock) Now() time.Time { return f.now }

func (f *fakeDemoClock) Sleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		f.now = f.now.Add(duration)
		return nil
	}
}

type recordedDemoPhase struct {
	demoPhaseExecution
	ClusterID string
}

type recordingDemoProgress struct {
	clusterID          string
	resolvedClusterIDs []string
	phases             []recordedDemoPhase
}

func (r *recordingDemoProgress) ClusterResolved(clusterID string) {
	r.clusterID = clusterID
	r.resolvedClusterIDs = append(r.resolvedClusterIDs, clusterID)
}

func (r *recordingDemoProgress) PhaseFinished(execution demoPhaseExecution) {
	r.phases = append(r.phases, recordedDemoPhase{demoPhaseExecution: execution, ClusterID: r.clusterID})
}

func demoRecordedPhaseNames(phases []recordedDemoPhase) []string {
	names := make([]string, 0, len(phases))
	for _, phase := range phases {
		names = append(names, phase.Phase)
	}
	return names
}

type inspectingDemoRunner struct {
	t          *testing.T
	valuesPath string
}

func (r *inspectingDemoRunner) LookPath(string) error { return nil }

func (r *inspectingDemoRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func (r *inspectingDemoRunner) RunUnlogged(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func (r *inspectingDemoRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	require.Equal(r.t, "helm", name)
	index := indexOf(args, "--values")
	require.GreaterOrEqual(r.t, index, 0)
	require.Greater(r.t, len(args), index+1)
	r.valuesPath = args[index+1]
	info, err := os.Stat(r.valuesPath)
	require.NoError(r.t, err)
	assert.Equal(r.t, os.FileMode(0600), info.Mode().Perm())
	content, err := os.ReadFile(r.valuesPath)
	require.NoError(r.t, err)
	assert.Equal(r.t, testDemoBootstrap().ValuesYAML, string(content))
	return nil, nil
}

type recordingDemoRunner struct {
	calls    [][]string
	unlogged [][]string
	outputs  [][]byte
	errors   []error
}

func (r *recordingDemoRunner) LookPath(string) error { return nil }

func (r *recordingDemoRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func (r *recordingDemoRunner) RunUnlogged(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.unlogged = append(r.unlogged, append([]string{name}, args...))
	return r.Run(ctx, name, args...)
}

func (r *recordingDemoRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	index := len(r.calls) - 1
	var output []byte
	var err error
	if index < len(r.outputs) {
		output = r.outputs[index]
	}
	if index < len(r.errors) {
		err = r.errors[index]
	}
	return output, err
}

func indexOf[T comparable](values []T, target T) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

func testDemoPlatformConfiguration(templateKey string, templateVersion string) *qovery.ClusterPlatformConfigurationResponse {
	return &qovery.ClusterPlatformConfigurationResponse{
		Platform:      *qovery.NewPlatformSelection(templateKey, templateVersion),
		ClusterInputs: map[string]map[string]string{},
	}
}

const testDemoPlatformCatalog = `{
	"results": [
		{"key": "qovery-cluster-v0", "version": "0.1.0", "status": "PUBLISHED", "description": null, "layers": []},
		{
			"key": "qovery-demo-v0",
			"version": "0.2.0",
			"status": "PUBLISHED",
			"description": "Local demo cluster",
			"bootstrapComponent": {
				"key": "qovery-operator",
				"kind": "HELM",
				"dependsOn": [],
				"description": null,
				"fields": [
					{
						"key": "cpuArchitectures",
						"type": "string",
						"required": true,
						"defaultValue": null,
						"label": "CPU architectures",
						"description": null,
						"sensitive": false,
						"constraints": {"allowedValues": ["AMD64", "ARM64"], "min": null, "max": null, "minLength": null, "maxLength": null, "pattern": null}
					}
				],
				"configurationSections": []
			},
			"layers": [
				{
					"key": "qovery-stack",
					"mandatory": false,
					"enabledByDefault": true,
					"modes": ["CUSTOMER_MANAGED"],
					"providers": null,
					"componentKeys": ["cert-manager"],
					"components": [
						{
							"key": "cert-manager",
							"kind": "HELM",
							"dependsOn": [],
							"description": null,
							"fields": [
								{
									"key": "email",
									"type": "string",
									"required": false,
									"defaultValue": null,
									"label": "ACME email",
									"description": null,
									"sensitive": false,
									"constraints": {"allowedValues": null, "min": null, "max": null, "minLength": null, "maxLength": null, "pattern": null}
								},
								{
									"key": "issuers",
									"type": "array",
									"required": false,
									"label": "Issuers",
									"description": null,
									"sensitive": false,
									"constraints": {"minItems": null, "maxItems": null, "uniqueItems": false},
									"items": {
										"type": "object",
										"fields": [
											{
												"key": "name",
												"type": "string",
												"required": true,
												"defaultValue": null,
												"label": "Name",
												"description": null,
												"sensitive": false,
												"constraints": {"allowedValues": null, "min": null, "max": null, "minLength": 1, "maxLength": null, "pattern": null}
											},
											{
												"key": "server",
												"type": "string",
												"required": true,
												"defaultValue": "https://acme-v02.api.letsencrypt.org/directory",
												"label": "ACME server",
												"description": null,
												"sensitive": false,
												"constraints": {"allowedValues": null, "min": null, "max": null, "minLength": null, "maxLength": null, "pattern": null}
											}
										]
									}
								}
							],
							"configurationSections": []
						}
					],
					"description": null
				},
				{
					"key": "dns-certificates",
					"mandatory": true,
					"enabledByDefault": true,
					"modes": ["CUSTOMER_MANAGED"],
					"providers": null,
					"componentKeys": [],
					"components": [],
					"description": null
				}
			]
		}
	]
}`

const testDemoDefaultPlatformConfigurationRequest = `{
	"platform": {
		"templateKey": "qovery-demo-v0",
		"templateVersion": "0.2.0",
		"layerSelections": {"qovery-stack": true},
		"managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}
	},
	"clusterInputs": {}
}`

type fakeDemoPlatformServer struct {
	t             *testing.T
	server        *httptest.Server
	configuration string
	mutex         sync.Mutex
	failures      map[string]fakeDemoPlatformFailure
	received      []string
	saved         string
}

type fakeDemoPlatformFailure struct {
	status int
	body   string
}

func newFakeDemoPlatformServer(t *testing.T, configuration string) *fakeDemoPlatformServer {
	fake := &fakeDemoPlatformServer{t: t, configuration: configuration, failures: map[string]fakeDemoPlatformFailure{}}
	fake.server = httptest.NewServer(fake)
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeDemoPlatformServer) fail(call string, status int, body string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.failures[call] = fakeDemoPlatformFailure{status: status, body: body}
}

func (f *fakeDemoPlatformServer) api() *demoQoveryAPI {
	configuration := qovery.NewConfiguration()
	configuration.Servers = qovery.ServerConfigurations{{URL: f.server.URL}}
	configuration.HTTPClient = f.server.Client()
	return &demoQoveryAPI{client: qovery.NewAPIClient(configuration)}
}

func (f *fakeDemoPlatformServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	call := request.Method + " " + request.URL.Path
	f.received = append(f.received, call)
	writer.Header().Set("Content-Type", "application/json")
	if failure, failed := f.failures[call]; failed {
		writer.WriteHeader(failure.status)
		_, _ = io.WriteString(writer, failure.body)
		return
	}
	switch call {
	case "GET /organization/organization-id/platformTemplate":
		_, _ = io.WriteString(writer, testDemoPlatformCatalog)
	case "GET /v1/cluster/cluster-id/platformConfiguration":
		if f.configuration == "" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(writer, f.configuration)
	case "PUT /v1/cluster/cluster-id/platformConfiguration":
		body, err := io.ReadAll(request.Body)
		if err != nil {
			f.t.Errorf("cannot read the saved platform configuration: %v", err)
		}
		f.saved = string(body)
		_, _ = io.WriteString(writer, `{"clusterId":"cluster-id","organizationId":"organization-id","platform":{"templateKey":"qovery-demo-v0","templateVersion":"0.2.0"},"clusterInputs":{},"layers":[]}`)
	default:
		f.t.Errorf("unexpected request %s", call)
		writer.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeDemoPlatformServer) requests() []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]string(nil), f.received...)
}

func (f *fakeDemoPlatformServer) savedConfiguration() string {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.saved
}

func TestDemoUpReportsEachSucceededPhase(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	api := &fakeDemoAPI{
		operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)},
		clusterStatuses:  []string{"READY", "DEPLOYED"},
	}
	var out bytes.Buffer
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: &fakeDemoClock{now: now}, out: &out}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Waiting for a fresh Qovery Operator heartbeat\n\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\n✅ Done (0s)\n")
	assert.Contains(t, out.String(), "Deploying the current self-managed platform catalog\n\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\n✅ Done (1s)\n")
	assert.Contains(t, out.String(), "To stop or start your cluster, use `k3d cluster stop|start local-demo-user`. To delete it, use `qovery demo destroy -c local-demo-user -d`.\n")
}

func TestDemoUpDoesNotReportAFailedPhaseAsDone(t *testing.T) {
	var out bytes.Buffer
	orchestrator := demoOrchestrator{api: &fakeDemoAPI{}, local: &fakeDemoLocal{legacyRelease: true}, clock: &fakeDemoClock{now: time.Now()}, out: &out}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	assert.True(t, strings.HasSuffix(out.String(), "Checking for an unsupported legacy Qovery release\n\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\n"))
}

func TestFindDemoClusterRefusesAClusterThatIsNotADemo(t *testing.T) {
	isDemo := true
	notDemo := false
	clusters := []qovery.Cluster{
		{Id: "demo-id", Name: "local-demo-user", IsDemo: &isDemo},
		{Id: "prod-id", Name: "production", IsDemo: &notDemo},
		{Id: "legacy-id", Name: "unflagged"},
	}

	id, found, err := findDemoCluster(clusters, "local-demo-user")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "demo-id", id)

	_, found, err = findDemoCluster(clusters, "missing")
	require.NoError(t, err)
	assert.False(t, found)

	for _, name := range []string{"production", "unflagged"} {
		_, found, err = findDemoCluster(clusters, name)
		require.Error(t, err)
		assert.False(t, found)
		assert.Contains(t, err.Error(), "is not a demo cluster")
	}
}

func TestDemoUpSuggestsAnotherNameWhenTheClusterIsNotADemo(t *testing.T) {
	_, _, notADemo := findDemoCluster([]qovery.Cluster{{Id: "prod-id", Name: "production"}}, "production")
	api := &fakeDemoAPI{findErr: notADemo}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemoConfig())

	require.Error(t, err)
	assert.Equal(t, `cluster "production" in this organization is not a demo cluster; qovery demo only manages demo clusters; choose another name with --cluster-name`, err.Error())
	assert.Equal(t, 0, api.deployCalls)
}

func TestDemoLegacyReleaseCheckTargetsTheDemoContext(t *testing.T) {
	runner := &recordingDemoRunner{outputs: [][]byte{[]byte("[]")}}
	local := demoLocalCommands{runner: runner, goos: "linux", kubeContext: "k3d-local-demo-user"}

	legacy, err := local.LegacyQoveryReleaseExists(context.Background())

	require.NoError(t, err)
	assert.False(t, legacy)
	assert.Equal(t, []string{"helm", "list", "--kube-context", "k3d-local-demo-user", "--namespace", "qovery", "--all", "--output", "json", "--filter", "^qovery$"}, runner.calls[0])
}

func TestLoopbackHasAddressMatchesTheExactIP(t *testing.T) {
	macOS := []byte("lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384\n\tinet 127.0.0.1 netmask 0xff000000\n\tinet 172.42.0.30 netmask 0xffffffff\n")
	linux := []byte("1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536\n    inet 127.0.0.1/8 scope host lo\n    inet 172.42.0.3/32 scope global lo\n")

	assert.False(t, loopbackHasAddress(macOS, demoNodeIP))
	assert.True(t, loopbackHasAddress(append(macOS, []byte("\tinet 172.42.0.3 netmask 0xffffffff\n")...), demoNodeIP))
	assert.True(t, loopbackHasAddress(linux, demoNodeIP))
	assert.False(t, loopbackHasAddress([]byte("inet 172.42.0.31/32 scope global lo"), demoNodeIP))
}

func TestDemoUpReportsAnInterruptedPhaseAsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	progress := &recordingDemoProgress{}
	events := []string{}
	api := &fakeDemoAPI{events: &events, operatorStatuses: []demoOperatorStatus{{Connected: false}}}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{events: &events}, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}, progress: progress}

	err := orchestrator.Up(ctx, testDemoConfig())

	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, progress.phases, 1)
	assert.Equal(t, demoPhaseLocalDependencies, progress.phases[0].Phase)
	assert.Equal(t, demoPhaseResultCanceled, progress.phases[0].Result)
	assert.Empty(t, progress.phases[0].ErrorCode)
	assert.Empty(t, events)
}

func TestDemoUpCancelsThePhaseRunningWhenInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	progress := &recordingDemoProgress{}
	api := &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{{Connected: false}}}
	clock := &cancelingDemoClock{fakeDemoClock: fakeDemoClock{now: time.Now()}, cancel: cancel}
	orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{}, clock: clock, out: &bytes.Buffer{}, progress: progress}

	err := orchestrator.Up(ctx, testDemoConfig())

	require.ErrorIs(t, err, context.Canceled)
	last := progress.phases[len(progress.phases)-1]
	assert.Equal(t, demoPhaseOperatorHeartbeat, last.Phase)
	assert.Equal(t, demoPhaseResultCanceled, last.Result)
	assert.Empty(t, last.ErrorCode)
}

type cancelingDemoClock struct {
	fakeDemoClock
	cancel context.CancelFunc
}

func (c *cancelingDemoClock) Sleep(ctx context.Context, duration time.Duration) error {
	c.cancel()
	return c.fakeDemoClock.Sleep(ctx, duration)
}

func TestCallDemoAPIRetriesOnceWithAFreshClientOnUnauthorized(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "qov_test-token")
	expired := utils.GetQoveryClient("Bearer", "expired")
	api := &demoQoveryAPI{client: expired}
	var clients []*qovery.APIClient
	call := func(client *qovery.APIClient) (*qovery.ClusterStatus, *http.Response, error) {
		clients = append(clients, client)
		if len(clients) == 1 {
			return nil, &http.Response{StatusCode: http.StatusUnauthorized}, errors.New("401 Unauthorized")
		}
		return &qovery.ClusterStatus{}, &http.Response{StatusCode: http.StatusOK}, nil
	}

	_, response, err := callDemoAPI(api, call)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	require.Len(t, clients, 2)
	assert.Same(t, expired, clients[0])
	assert.NotSame(t, expired, clients[1])
	assert.Same(t, clients[1], api.client)
}

func TestCallDemoAPIDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	_, _, err := callDemoAPI(&demoQoveryAPI{}, func(*qovery.APIClient) (*qovery.ClusterStatus, *http.Response, error) {
		calls++
		return nil, &http.Response{StatusCode: http.StatusInternalServerError}, errors.New("500")
	})

	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestCallDemoAPIRejectsAnEmptySuccessfulResponse(t *testing.T) {
	_, _, err := callDemoAPI(&demoQoveryAPI{}, func(*qovery.APIClient) (*qovery.ClusterResponseList, *http.Response, error) {
		return nil, &http.Response{StatusCode: http.StatusOK}, nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestOperatorStatusReadyToleratesClockSkewButRejectsStaleHeartbeats(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	at := func(offset time.Duration) demoOperatorStatus {
		heartbeat := now.Add(offset)
		return demoOperatorStatus{Connected: true, LastHeartbeat: &heartbeat}
	}

	assert.True(t, operatorStatusReady(at(90*time.Second), now))
	assert.True(t, operatorStatusReady(at(-time.Minute), now))
	assert.False(t, operatorStatusReady(at(-3*time.Minute), now))
	assert.False(t, operatorStatusReady(demoOperatorStatus{Connected: false, LastHeartbeat: at(0).LastHeartbeat}, now))
}
