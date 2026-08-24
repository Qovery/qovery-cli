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
	qovery "github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemo2UpFreshCreationOrder(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemo2API{
		events:           &events,
		operatorStatuses: []demo2OperatorStatus{readyDemo2OperatorStatus(now)},
		clusterStatuses:  []string{"READY", "DEPLOYED"},
	}
	local := &fakeDemo2Local{events: &events}
	orchestrator := demo2Orchestrator{api: api, local: local, clock: &fakeDemo2Clock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.NoError(t, err)
	assert.Equal(t, []string{
		"dependencies", "credentials", "find-cluster", "create-cluster", "k3d", "loopback",
		"legacy-release", "operator-config", "bootstrap", "attach", "install-operator", "operator-status",
		"deploy", "cluster-status", "cluster-status", "validate-workloads",
	}, events)
	assert.Equal(t, "ARM64", api.operatorCPUArchitecture)
}

func TestNewDemo2AttemptUsesUUIDV7(t *testing.T) {
	attempt := newDemo2Attempt()
	attemptID, err := uuid.Parse(attempt.id)

	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), attemptID.Version())
}

func TestDemo2UpRerunReusesResources(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemo2API{
		events:           &events,
		clusterFound:     true,
		operatorStatuses: []demo2OperatorStatus{readyDemo2OperatorStatus(now)},
		clusterStatuses:  []string{"RESTARTED"},
	}
	local := &fakeDemo2Local{events: &events}
	orchestrator := demo2Orchestrator{api: api, local: local, clock: &fakeDemo2Clock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.NoError(t, err)
	assert.NotContains(t, events, "create-cluster")
	assert.Contains(t, events, "install-operator")
	assert.Contains(t, events, "deploy")
	assert.Equal(t, 1, api.ensureCredentialsCalls)
}

func TestDemo2ClusterRequestSerializesIsDemo(t *testing.T) {
	request := newDemo2ClusterRequest("local-demo2-user", demo2Credential{ID: "credential-id", Name: "on-premise"})

	payload, err := json.Marshal(request)

	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, true, decoded["is_demo"])
	assert.Equal(t, "ON_PREMISE", decoded["cloud_provider"])
	assert.Equal(t, "SELF_MANAGED", decoded["kubernetes"])
	assert.Equal(t, false, decoded["production"])
}

func TestDemo2OperatorConfigurationPreservesExistingConfiguration(t *testing.T) {
	configuration := testDemo2PlatformConfiguration(demo2PlatformTemplateKey, "0.1.0")
	configuration.Platform.SetLayerSelections(map[string]bool{"qovery-stack": false})
	configuration.Platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {"existing": "value"},
	})
	configuration.ClusterInputs = map[string]map[string]string{"qovery-operator": {"input": "value"}}

	request, err := newDemo2OperatorConfigurationRequest(configuration, "ARM64")

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"qovery-stack": false}, request.Platform.GetLayerSelections())
	assert.Equal(t, map[string]map[string]interface{}{
		"qovery-operator": {"existing": "value", "cpuArchitectures": "ARM64"},
	}, request.Platform.GetManagedConfig())
	assert.Equal(t, map[string]map[string]string{"qovery-operator": {"input": "value"}}, request.ClusterInputs)
}

func TestDemo2OperatorConfigurationRequestAlwaysSendsClusterInputs(t *testing.T) {
	configuration := &qovery.ClusterPlatformConfigurationResponse{
		Platform: *qovery.NewPlatformSelection(demo2PlatformTemplateKey, "0.1.0"),
	}

	request, err := newDemo2OperatorConfigurationRequest(configuration, "AMD64")

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

func TestDemo2PlatformConfigurationSelectsTheDemoTemplate(t *testing.T) {
	catalog := qovery.NewPlatformTemplateCatalogResponse([]qovery.PlatformTemplateSummaryResponse{
		{
			Key:     "qovery-cluster-v0",
			Version: "0.1.0",
		},
		{
			Key:     demo2PlatformTemplateKey,
			Version: "0.2.0",
			Layers: []qovery.PlatformTemplateLayerResponse{
				{Key: "qovery-stack", Mandatory: false, EnabledByDefault: true},
				{Key: "dns-certificates", Mandatory: true, EnabledByDefault: true},
			},
		},
	})

	configuration, err := newDemo2PlatformConfiguration(catalog)

	require.NoError(t, err)
	assert.Equal(t, demo2PlatformTemplateKey, configuration.Platform.TemplateKey)
	assert.Equal(t, "0.2.0", configuration.Platform.TemplateVersion)
	assert.Equal(t, map[string]bool{"qovery-stack": true}, configuration.Platform.GetLayerSelections())
	assert.Empty(t, configuration.Platform.GetManagedConfig())
	assert.Empty(t, configuration.ClusterInputs)
}

func TestDemo2PlatformConfigurationRequiresTheDemoTemplate(t *testing.T) {
	catalog := qovery.NewPlatformTemplateCatalogResponse([]qovery.PlatformTemplateSummaryResponse{{
		Key:     "qovery-cluster-v0",
		Version: "0.1.0",
	}})

	_, err := newDemo2PlatformConfiguration(catalog)

	require.EqualError(t, err, `qovery returned no "qovery-demo-v0" platform template for the local demo`)
}

func TestDemo2PlatformCatalogFixtureExercisesFieldSchemaVariants(t *testing.T) {
	var catalog qovery.PlatformTemplateCatalogResponse
	require.NoError(t, json.Unmarshal([]byte(testDemo2PlatformCatalog), &catalog))
	require.Len(t, catalog.Results, 2)
	demo := catalog.Results[1]
	bootstrap := demo.GetBootstrapComponent()
	require.Len(t, bootstrap.Fields, 1)
	fields := demo.Layers[0].Components[0].Fields
	require.Len(t, fields, 2)

	assert.Equal(t, demo2OperatorComponentKey, bootstrap.Key)
	assert.NotNil(t, bootstrap.Fields[0].ScalarFieldSchemaResponse)
	assert.NotNil(t, fields[0].ScalarFieldSchemaResponse)
	require.NotNil(t, fields[1].ArrayFieldSchemaResponse)
	require.NotNil(t, fields[1].ArrayFieldSchemaResponse.Items.ObjectArrayItemResponse)
	assert.Len(t, fields[1].ArrayFieldSchemaResponse.Items.ObjectArrayItemResponse.Fields, 2)
}

func TestSelectDemo2PlatformConfigurationPreservesMatchingConfiguration(t *testing.T) {
	existing := testDemo2PlatformConfiguration(demo2PlatformTemplateKey, "0.1.0")
	existing.Platform.SetManagedConfig(map[string]map[string]interface{}{"qovery-operator": {"existing": "value"}})
	desired := testDemo2PlatformConfiguration(demo2PlatformTemplateKey, "0.1.0")

	selected := selectDemo2PlatformConfiguration(existing, desired)

	assert.Same(t, existing, selected)
}

func TestSelectDemo2PlatformConfigurationReplacesStandardConfiguration(t *testing.T) {
	existing := testDemo2PlatformConfiguration("qovery-cluster-v0", "0.1.0")
	existing.Platform.SetLayerSelections(map[string]bool{"log-infra": false})
	desired := testDemo2PlatformConfiguration(demo2PlatformTemplateKey, "0.1.0")
	desired.Platform.SetLayerSelections(map[string]bool{})

	selected := selectDemo2PlatformConfiguration(existing, desired)

	assert.Same(t, desired, selected)
	assert.NotContains(t, selected.Platform.GetLayerSelections(), "log-infra")
}

func TestDemo2PlatformConfigurationRejectsRedactedValuesItWouldSendBack(t *testing.T) {
	configuration := testDemo2PlatformConfiguration(demo2PlatformTemplateKey, "0.1.0")
	configuration.Platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {"cpuArchitectures": "<redacted>"},
		"karpenter":       {"replicas": 2, "token": "<redacted>"},
		"cert-manager":    {"email": "<redacted>", "issuer": "<redacted>"},
		"external-dns":    {"provider": "cloudflare"},
	})
	request, err := newDemo2OperatorConfigurationRequest(configuration, "ARM64")
	require.NoError(t, err)

	err = ensureDemo2RequestHasNoRedactedValues(request)

	require.EqualError(t, err, "qovery returned redacted values for cert-manager.email, cert-manager.issuer, karpenter.token; demo2 sends the whole configuration back and would store the redaction marker instead, so nothing was saved. Retry later; if it persists, these fields are sensitive and demo2 cannot keep them: clear them, or recreate the cluster with `qovery demo2 destroy --delete-qovery-config`")
}

func TestNormalizeDemo2CPUArchitecture(t *testing.T) {
	arm64, err := normalizeDemo2CPUArchitecture(" arm64 ")
	require.NoError(t, err)
	amd64, err := normalizeDemo2CPUArchitecture("AMD64")
	require.NoError(t, err)
	_, err = normalizeDemo2CPUArchitecture("386")

	assert.Equal(t, "ARM64", arm64)
	assert.Equal(t, "AMD64", amd64)
	require.EqualError(t, err, `unsupported local CPU architecture "386"`)
}

func TestDemo2PlatformConfigurationAcceptsUnredactedValues(t *testing.T) {
	platform := qovery.NewPlatformSelection(demo2PlatformTemplateKey, "0.1.0")
	platform.SetManagedConfig(map[string]map[string]interface{}{
		"qovery-operator": {
			"cpuArchitectures": "ARM64",
			"note":             "redacted",
			"nested":           map[string]interface{}{"token": "<redacted>"},
		},
	})
	request := qovery.NewClusterPlatformConfigurationRequest(*platform, map[string]map[string]string{})

	require.NoError(t, ensureDemo2RequestHasNoRedactedValues(*request))
}

func TestDemo2ConfigureOperatorCreatesTheDemoConfiguration(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, "")

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"GET /organization/organization-id/platformTemplate",
		"GET /v1/cluster/cluster-id/platformConfiguration",
		"PUT /v1/cluster/cluster-id/platformConfiguration",
	}, server.requests())
	assert.JSONEq(t, testDemo2DefaultPlatformConfigurationRequest, server.savedConfiguration())
}

func TestDemo2ConfigureOperatorReplacesAnotherRedactedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name            string
		templateKey     string
		templateVersion string
	}{
		{name: "another template", templateKey: "qovery-cluster-v0", templateVersion: "0.2.0"},
		{name: "another version", templateKey: demo2PlatformTemplateKey, templateVersion: "0.1.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeDemo2PlatformServer(t, fmt.Sprintf(`{
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
			assert.JSONEq(t, testDemo2DefaultPlatformConfigurationRequest, server.savedConfiguration())
		})
	}
}

func TestDemo2ConfigureOperatorOverwritesARedactedCPUArchitecture(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, `{
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

func TestDemo2ConfigureOperatorUpdatesTheExistingDemoConfiguration(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, `{
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

func TestDemo2ConfigureOperatorDoesNotSaveRedactedConfiguration(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, `{
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

func TestDemo2ConfigureOperatorSkipsAnUpToDateConfiguration(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, `{
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

func TestDemo2ConfigureOperatorRejectsAnUnsupportedArchitectureBeforeCallingQovery(t *testing.T) {
	server := newFakeDemo2PlatformServer(t, "")

	err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "386")

	require.EqualError(t, err, `unsupported local CPU architecture "386"`)
	assert.Empty(t, server.requests())
}

func TestDemo2ConfigureOperatorReturnsQoveryErrors(t *testing.T) {
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
			server := newFakeDemo2PlatformServer(t, "")
			server.fail(test.call, test.status, test.body)

			err := server.api().ConfigureOperator(context.Background(), "organization-id", "cluster-id", "arm64")

			require.EqualError(t, err, test.error)
			assert.Equal(t, test.requests, server.requests())
		})
	}
}

func TestDemo2UpRejectsLegacyQoveryRelease(t *testing.T) {
	events := []string{}
	api := &fakeDemo2API{events: &events}
	local := &fakeDemo2Local{events: &events, legacyRelease: true}
	orchestrator := demo2Orchestrator{api: api, local: local, clock: &fakeDemo2Clock{now: time.Now()}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "adopting an old demo is not supported")
	assert.NotContains(t, events, "bootstrap")
	assert.NotContains(t, events, "install-operator")
}

func TestBuildDemo2HelmArgsUsesStructuredBootstrap(t *testing.T) {
	bootstrap := testDemo2Bootstrap()

	args, err := buildDemo2HelmArgs(bootstrap, "/tmp/protected-values.yaml")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"upgrade", "--install", "qovery-operator", "oci://registry.example/qovery-operator",
		"--version", "1.2.3", "--namespace", "qovery", "--values", "/tmp/protected-values.yaml",
		"--create-namespace", "--atomic", "--wait", "--timeout", "15m",
	}, args)
	assert.NotContains(t, strings.Join(args, " "), "ignored helm command")
}

func TestBuildDemo2HelmArgsRejectsUmbrellaChart(t *testing.T) {
	bootstrap := testDemo2Bootstrap()
	bootstrap.ChartReference = "oci://public.ecr.aws/example/charts/qovery"

	_, err := buildDemo2HelmArgs(bootstrap, "/tmp/protected-values.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "umbrella chart")
}

func TestDemo2OperatorValuesFileIsProtectedAndRemoved(t *testing.T) {
	runner := &inspectingDemo2Runner{t: t}
	local := demo2LocalCommands{runner: runner, goos: "linux", tempDir: t.TempDir()}

	err := local.InstallOperator(context.Background(), testDemo2Bootstrap())

	require.NoError(t, err)
	require.NotEmpty(t, runner.valuesPath)
	_, statErr := os.Stat(runner.valuesPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDemo2ExecRunnerKeepsCommandOutputInTheDebugLog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo2 is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demo2ExecRunner{out: &terminal, log: &log}

	_, err := runner.RunQuiet(context.Background(), "/bin/sh", "-c", "printf diagnostic >&2; exit 7")

	require.Error(t, err)
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "$ /bin/sh -c")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemo2ExecRunnerStreamsSuccessfulCommandsInDebugMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo2 is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demo2ExecRunner{out: &terminal, log: &log, debug: true}

	_, err := runner.RunQuiet(context.Background(), "/bin/sh", "-c", "printf diagnostic")

	require.NoError(t, err)
	assert.Contains(t, terminal.String(), "$ /bin/sh -c")
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemo2ExecRunnerStreamsActionCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo2 is not supported directly on Windows")
	}
	var terminal bytes.Buffer
	var log bytes.Buffer
	runner := demo2ExecRunner{out: &terminal, log: &log}

	_, err := runner.Run(context.Background(), "/bin/sh", "-c", "printf diagnostic")

	require.NoError(t, err)
	assert.Contains(t, terminal.String(), "$ /bin/sh -c")
	assert.Contains(t, terminal.String(), "diagnostic")
	assert.Contains(t, log.String(), "diagnostic")
}

func TestDemo2UpTimesOutWaitingForOperator(t *testing.T) {
	clock := &fakeDemo2Clock{now: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)}
	api := &fakeDemo2API{operatorStatuses: []demo2OperatorStatus{{Connected: false}}}
	local := &fakeDemo2Local{}
	orchestrator := demo2Orchestrator{api: api, local: local, clock: clock, out: &bytes.Buffer{}}
	cfg := testDemo2Config()
	cfg.OperatorTimeout = 2 * time.Second
	cfg.PollInterval = time.Second

	err := orchestrator.Up(context.Background(), cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Equal(t, 0, api.deployCalls)
}

func TestDemo2UpDoesNotDeployBeforeHeartbeat(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	events := []string{}
	api := &fakeDemo2API{
		events: &events,
		operatorStatuses: []demo2OperatorStatus{
			{Connected: true},
			readyDemo2OperatorStatus(now.Add(time.Second)),
		},
		clusterStatuses: []string{"DEPLOYED"},
	}
	orchestrator := demo2Orchestrator{api: api, local: &fakeDemo2Local{events: &events}, clock: &fakeDemo2Clock{now: now}, out: &bytes.Buffer{}}
	cfg := testDemo2Config()
	cfg.PollInterval = time.Second

	err := orchestrator.Up(context.Background(), cfg)

	require.NoError(t, err)
	firstStatus := indexOf(events, "operator-status")
	secondStatus := indexOf(events[firstStatus+1:], "operator-status") + firstStatus + 1
	deploy := indexOf(events, "deploy")
	assert.Greater(t, deploy, secondStatus)
}

func TestDemo2UpDeploymentSucceedsOnlyOnDeployed(t *testing.T) {
	now := time.Now()
	api := &fakeDemo2API{
		operatorStatuses: []demo2OperatorStatus{readyDemo2OperatorStatus(now)},
		clusterStatuses:  []string{"READY", "DEPLOYING", "DEPLOYED"},
	}
	orchestrator := demo2Orchestrator{api: api, local: &fakeDemo2Local{}, clock: &fakeDemo2Clock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.NoError(t, err)
	assert.Equal(t, 3, api.clusterStatusCalls)
}

func TestDemo2UpReturnsTerminalDeploymentError(t *testing.T) {
	now := time.Now()
	api := &fakeDemo2API{
		operatorStatuses: []demo2OperatorStatus{readyDemo2OperatorStatus(now)},
		clusterStatuses:  []string{"DEPLOYING", "DEPLOYMENT_ERROR"},
	}
	orchestrator := demo2Orchestrator{api: api, local: &fakeDemo2Local{}, clock: &fakeDemo2Clock{now: now}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEPLOYMENT_ERROR")
}

func TestDemo2UpRedactsValuesFromErrors(t *testing.T) {
	secret := "super-secret-cluster-jwt"
	api := &fakeDemo2API{bootstrap: demo2Bootstrap{
		ReleaseName: "qovery-operator", ChartReference: "chart", ChartVersion: "1", Namespace: "qovery", ValuesYAML: "token: " + secret,
	}}
	local := &fakeDemo2Local{installErr: errors.New("helm failed with values_yaml token: " + secret)}
	orchestrator := demo2Orchestrator{api: api, local: local, clock: &fakeDemo2Clock{now: time.Now()}, out: &bytes.Buffer{}}

	err := orchestrator.Up(context.Background(), testDemo2Config())

	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, err.Error(), "values_yaml")
	assert.Contains(t, err.Error(), "redacted")
}

func TestDemo2EnsureK3dClusterUsesPinnedSubstrate(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte("[]"),
		nil,
		[]byte(`[{"name":"qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo2-user":{"DNSNames":["qovery-registry.lan"]}}}}]`),
	}}
	local := demo2LocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo2-user")

	require.NoError(t, err)
	require.Len(t, runner.calls, 4)
	assert.Equal(t, "k3d", runner.calls[1][0])
	joined := strings.Join(runner.calls[1][1:], " ")
	assert.Contains(t, joined, demo2K3sImage)
	assert.Contains(t, joined, demo2Subnet)
	assert.Contains(t, joined, "--node-ip="+demo2NodeIP+"@server:0")
	assert.Contains(t, joined, "--disable=traefik@server:*")
	assert.Contains(t, joined, demo2Registry)
	assert.Contains(t, joined, "80:80@loadbalancer")
	assert.Contains(t, joined, "443:443@loadbalancer")
}

func TestDemo2EnsureK3dClusterStartsExistingCluster(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo2-user"}]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo2-user":{"DNSNames":["qovery-registry.lan"]}}}}]`),
	}}
	local := demo2LocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo2-user")

	require.NoError(t, err)
	assert.Equal(t, []string{"k3d", "cluster", "start", "local-demo2-user"}, runner.calls[1])
	assert.Equal(t, []string{"k3d", "registry", "list", "--output", "json"}, runner.calls[2])
}

func TestDemo2EnsureK3dClusterRecreatesMissingRegistry(t *testing.T) {
	runner := &recordingDemo2Runner{outputs: [][]byte{
		[]byte(`[{"name":"local-demo2-user"}]`),
		nil,
		[]byte(`[]`),
		nil,
		[]byte(`[{"name":"k3d-qovery-registry.lan"}]`),
		[]byte(`[{"NetworkSettings":{"Networks":{"k3d-local-demo2-user":{"DNSNames":["k3d-qovery-registry.lan"]}}}}]`),
		nil,
		nil,
	}}
	local := demo2LocalCommands{runner: runner, goos: "linux"}

	err := local.EnsureK3dCluster(context.Background(), "local-demo2-user")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"k3d", "registry", "create", demo2Registry,
		"--default-network", "k3d-local-demo2-user",
		"--no-help",
	}, runner.calls[3])
	assert.Equal(t, []string{
		"docker", "network", "connect", "--alias", demo2Registry,
		"k3d-local-demo2-user", "k3d-" + demo2Registry,
	}, runner.calls[7])
}

func TestDemo2ValidateWorkloadsRequiresOperatorAndRejectsPermanentEngine(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		runner := &recordingDemo2Runner{outputs: [][]byte{[]byte(`{"items":[{"metadata":{"name":"qovery-operator"}}]}`)}}
		local := demo2LocalCommands{runner: runner, goos: "linux"}
		require.NoError(t, local.ValidateWorkloads(context.Background(), "qovery"))
	})

	t.Run("permanent engine", func(t *testing.T) {
		runner := &recordingDemo2Runner{outputs: [][]byte{[]byte(`{"items":[{"metadata":{"name":"qovery-operator"}},{"metadata":{"name":"qovery-engine"}}]}`)}}
		local := demo2LocalCommands{runner: runner, goos: "linux"}
		err := local.ValidateWorkloads(context.Background(), "qovery")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected permanent Deployment qovery-engine")
	})
}

func testDemo2Config() demo2Config {
	return demo2Config{
		OrganizationID:    "organization-id",
		ClusterName:       "local-demo2-user",
		CPUArchitecture:   "ARM64",
		OperatorTimeout:   time.Minute,
		DeploymentTimeout: time.Minute,
		PollInterval:      time.Second,
	}
}

func testDemo2Bootstrap() demo2Bootstrap {
	return demo2Bootstrap{
		ReleaseName:    "qovery-operator",
		ChartReference: "oci://registry.example/qovery-operator",
		ChartVersion:   "1.2.3",
		Namespace:      "qovery",
		ValuesYAML:     "secretToken: highly-sensitive\n",
	}
}

func readyDemo2OperatorStatus(now time.Time) demo2OperatorStatus {
	heartbeat := now
	return demo2OperatorStatus{Connected: true, LastHeartbeat: &heartbeat}
}

type fakeDemo2API struct {
	events                  *[]string
	clusterFound            bool
	bootstrap               demo2Bootstrap
	operatorStatuses        []demo2OperatorStatus
	clusterStatuses         []string
	ensureCredentialsCalls  int
	operatorStatusCalls     int
	clusterStatusCalls      int
	deployCalls             int
	deployStatus            string
	operatorCPUArchitecture string
}

func (f *fakeDemo2API) event(value string) {
	if f.events != nil {
		*f.events = append(*f.events, value)
	}
}

func (f *fakeDemo2API) EnsureOnPremiseCredentials(context.Context, string) (demo2Credential, error) {
	f.event("credentials")
	f.ensureCredentialsCalls++
	return demo2Credential{ID: "credential-id", Name: "on-premise"}, nil
}

func (f *fakeDemo2API) FindCluster(context.Context, string, string) (string, bool, error) {
	f.event("find-cluster")
	return "cluster-id", f.clusterFound, nil
}

func (f *fakeDemo2API) CreateCluster(context.Context, string, string, demo2Credential) (string, error) {
	f.event("create-cluster")
	return "cluster-id", nil
}

func (f *fakeDemo2API) ConfigureOperator(_ context.Context, _ string, _ string, cpuArchitecture string) error {
	f.event("operator-config")
	f.operatorCPUArchitecture = cpuArchitecture
	return nil
}

func (f *fakeDemo2API) GetOperatorBootstrap(context.Context, string, string) (demo2Bootstrap, error) {
	f.event("bootstrap")
	if f.bootstrap.ReleaseName != "" {
		return f.bootstrap, nil
	}
	return testDemo2Bootstrap(), nil
}

func (f *fakeDemo2API) AttachOperator(context.Context, string, string) error {
	f.event("attach")
	return nil
}

func (f *fakeDemo2API) GetOperatorStatus(context.Context, string, string) (demo2OperatorStatus, error) {
	f.event("operator-status")
	index := f.operatorStatusCalls
	f.operatorStatusCalls++
	if len(f.operatorStatuses) == 0 {
		return demo2OperatorStatus{}, nil
	}
	if index >= len(f.operatorStatuses) {
		index = len(f.operatorStatuses) - 1
	}
	return f.operatorStatuses[index], nil
}

func (f *fakeDemo2API) DeployCluster(context.Context, string, string) (string, error) {
	f.event("deploy")
	f.deployCalls++
	if f.deployStatus == "" {
		return "DEPLOYMENT_QUEUED", nil
	}
	return f.deployStatus, nil
}

func (f *fakeDemo2API) GetClusterStatus(context.Context, string, string) (string, error) {
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

type fakeDemo2Local struct {
	events        *[]string
	legacyRelease bool
	installErr    error
}

func (f *fakeDemo2Local) event(value string) {
	if f.events != nil {
		*f.events = append(*f.events, value)
	}
}

func (f *fakeDemo2Local) CheckDependencies(context.Context) error {
	f.event("dependencies")
	return nil
}

func (f *fakeDemo2Local) EnsureK3dCluster(context.Context, string) error {
	f.event("k3d")
	return nil
}

func (f *fakeDemo2Local) EnsureLoopback(context.Context) error {
	f.event("loopback")
	return nil
}

func (f *fakeDemo2Local) LegacyQoveryReleaseExists(context.Context) (bool, error) {
	f.event("legacy-release")
	return f.legacyRelease, nil
}

func (f *fakeDemo2Local) InstallOperator(context.Context, demo2Bootstrap) error {
	f.event("install-operator")
	return f.installErr
}

func (f *fakeDemo2Local) ValidateWorkloads(context.Context, string) error {
	f.event("validate-workloads")
	return nil
}

type fakeDemo2Clock struct {
	now time.Time
}

func (f *fakeDemo2Clock) Now() time.Time { return f.now }

func (f *fakeDemo2Clock) Sleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		f.now = f.now.Add(duration)
		return nil
	}
}

type inspectingDemo2Runner struct {
	t          *testing.T
	valuesPath string
}

func (r *inspectingDemo2Runner) LookPath(string) error { return nil }

func (r *inspectingDemo2Runner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func (r *inspectingDemo2Runner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
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
	assert.Equal(r.t, testDemo2Bootstrap().ValuesYAML, string(content))
	return nil, nil
}

type recordingDemo2Runner struct {
	calls   [][]string
	outputs [][]byte
	errors  []error
}

func (r *recordingDemo2Runner) LookPath(string) error { return nil }

func (r *recordingDemo2Runner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func (r *recordingDemo2Runner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
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

func testDemo2PlatformConfiguration(templateKey string, templateVersion string) *qovery.ClusterPlatformConfigurationResponse {
	return &qovery.ClusterPlatformConfigurationResponse{
		Platform:      *qovery.NewPlatformSelection(templateKey, templateVersion),
		ClusterInputs: map[string]map[string]string{},
	}
}

const testDemo2PlatformCatalog = `{
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

const testDemo2DefaultPlatformConfigurationRequest = `{
	"platform": {
		"templateKey": "qovery-demo-v0",
		"templateVersion": "0.2.0",
		"layerSelections": {"qovery-stack": true},
		"managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}
	},
	"clusterInputs": {}
}`

type fakeDemo2PlatformServer struct {
	t             *testing.T
	server        *httptest.Server
	configuration string
	mutex         sync.Mutex
	failures      map[string]fakeDemo2PlatformFailure
	received      []string
	saved         string
}

type fakeDemo2PlatformFailure struct {
	status int
	body   string
}

func newFakeDemo2PlatformServer(t *testing.T, configuration string) *fakeDemo2PlatformServer {
	fake := &fakeDemo2PlatformServer{t: t, configuration: configuration, failures: map[string]fakeDemo2PlatformFailure{}}
	fake.server = httptest.NewServer(fake)
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeDemo2PlatformServer) fail(call string, status int, body string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.failures[call] = fakeDemo2PlatformFailure{status: status, body: body}
}

func (f *fakeDemo2PlatformServer) api() *demo2QoveryAPI {
	configuration := qovery.NewConfiguration()
	configuration.Servers = qovery.ServerConfigurations{{URL: f.server.URL}}
	configuration.HTTPClient = f.server.Client()
	return &demo2QoveryAPI{client: qovery.NewAPIClient(configuration)}
}

func (f *fakeDemo2PlatformServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
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
		_, _ = io.WriteString(writer, testDemo2PlatformCatalog)
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

func (f *fakeDemo2PlatformServer) requests() []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]string(nil), f.received...)
}

func (f *fakeDemo2PlatformServer) savedConfiguration() string {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.saved
}
