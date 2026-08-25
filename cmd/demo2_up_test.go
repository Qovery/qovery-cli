package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestDemo2OperatorBindingPreservesExistingConfiguration(t *testing.T) {
	binding := &qovery.ClusterPlatformBindingResponse{
		TemplateKey:     demo2PlatformTemplateKey,
		TemplateVersion: "0.1.0",
		LayerSelections: map[string]bool{},
		ManagedConfig: map[string]map[string]interface{}{
			"qovery-operator": {"existing": "value"},
		},
		CustomerProvidedInputs: map[string]map[string]string{},
	}

	request, err := newDemo2OperatorBindingRequest(binding, "arm64")

	require.NoError(t, err)
	assert.Empty(t, request.GetLayerSelections())
	assert.Equal(t, "value", request.GetManagedConfig()["qovery-operator"]["existing"])
	assert.Equal(t, "ARM64", request.GetManagedConfig()["qovery-operator"]["cpuArchitectures"])
	assert.Empty(t, request.GetCustomerProvidedInputs())
}

func TestDemo2PlatformBindingSelectsTheDemoTemplate(t *testing.T) {
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

	binding, err := newDemo2PlatformBinding(catalog)

	require.NoError(t, err)
	assert.Equal(t, demo2PlatformTemplateKey, binding.TemplateKey)
	assert.Equal(t, "0.2.0", binding.TemplateVersion)
	assert.Equal(t, map[string]bool{"qovery-stack": true}, binding.LayerSelections)
	assert.Empty(t, binding.ManagedConfig)
	assert.Empty(t, binding.CustomerProvidedInputs)
}

func TestDemo2PlatformBindingRequiresTheDemoTemplate(t *testing.T) {
	catalog := qovery.NewPlatformTemplateCatalogResponse([]qovery.PlatformTemplateSummaryResponse{{
		Key:     "qovery-cluster-v0",
		Version: "0.1.0",
	}})

	_, err := newDemo2PlatformBinding(catalog)

	require.EqualError(t, err, `qovery returned no "qovery-demo-v0" platform template for the local demo`)
}

func TestSelectDemo2PlatformBindingPreservesMatchingBinding(t *testing.T) {
	existing := &qovery.ClusterPlatformBindingResponse{
		TemplateKey:     demo2PlatformTemplateKey,
		TemplateVersion: "0.1.0",
		ManagedConfig:   map[string]map[string]interface{}{"qovery-operator": {"existing": "value"}},
	}
	desired := &qovery.ClusterPlatformBindingResponse{
		TemplateKey:     demo2PlatformTemplateKey,
		TemplateVersion: "0.1.0",
	}

	selected := selectDemo2PlatformBinding(existing, desired)

	assert.Same(t, existing, selected)
}

func TestSelectDemo2PlatformBindingReplacesStandardBinding(t *testing.T) {
	existing := &qovery.ClusterPlatformBindingResponse{
		TemplateKey:     "qovery-cluster-v0",
		TemplateVersion: "0.1.0",
		LayerSelections: map[string]bool{"log-infra": false},
	}
	desired := &qovery.ClusterPlatformBindingResponse{
		TemplateKey:     demo2PlatformTemplateKey,
		TemplateVersion: "0.1.0",
		LayerSelections: map[string]bool{},
	}

	selected := selectDemo2PlatformBinding(existing, desired)

	assert.Same(t, desired, selected)
	assert.NotContains(t, selected.LayerSelections, "log-infra")
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
