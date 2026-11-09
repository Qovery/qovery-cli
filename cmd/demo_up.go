package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"github.com/tonistiigi/go-rosetta"
)

const (
	demoK3sImage            = "docker.io/rancher/k3s:v1.36.4-k3s1"
	demoSubnet              = "172.42.0.0/16"
	demoNodeIP              = "172.42.0.3"
	demoRegistry            = "qovery-registry.lan"
	demoPlatformTemplateKey = "qovery-demo-v0"
	demoWorkflowType        = "demo_installation"
	demoImplementation      = "engine_v2"
	demoUnknownErrorCode    = "UNKNOWN_FAILURE"
	demoSafeErrorMessage    = "The demo installation failed."
	// Leaves time for helm --atomic to roll back after an interrupt.
	demoCommandInterruptGrace = 2 * time.Minute
)

var (
	demoClusterName string
	demoDebug       bool
)

type demoAttempt struct {
	id          string
	startedAt   time.Time
	clusterID   string
	failedPhase string
	canceled    bool
}

type demoAttemptTelemetry struct {
	attempt *demoAttempt
	command *cobra.Command
}

var demoUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Create a local k3d cluster running the Qovery Operator and Engine v2",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		attempt := newDemoAttempt()
		attempt.capture(cmd, utils.DefaultEventName, utils.CommandExecutionProperties{})

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		context.AfterFunc(ctx, stop)
		err := runDemoUp(ctx, cmd, &demoAttemptTelemetry{attempt: attempt, command: cmd}, attempt.id)
		stop()
		if err != nil && (attempt.canceled || errors.Is(err, errDemoInterrupted)) {
			attempt.capture(cmd, utils.EndOfExecutionEventName, utils.CommandExecutionProperties{
				Result: demoPhaseResultCanceled,
				Phase:  attempt.failedPhase,
			})
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "\nDemo installation interrupted.")
			os.Exit(130)
		}
		if err != nil {
			errorCode, safeMessage := demoFailureDetails(err)
			attempt.capture(
				cmd,
				utils.EndOfExecutionErrorEventName,
				utils.CommandExecutionProperties{
					Result:           "failed",
					Phase:            attempt.failedPhase,
					ErrorCode:        errorCode,
					SafeErrorMessage: safeMessage,
				},
			)
			attempt.printFailureSummary(cmd, safeMessage)
			utils.PrintlnError(err)
			os.Exit(1)
		}

		attempt.capture(cmd, utils.EndOfExecutionEventName, utils.CommandExecutionProperties{Result: "succeeded"})
	},
}

func runDemoUp(ctx context.Context, cmd *cobra.Command, progress demoProgress, diagnosticID string) error {
	if runtime.GOOS == "windows" {
		return newDemoFailure(
			demoErrorUnsupportedOS,
			"Windows is not supported directly; use WSL.",
			errors.New("qovery demo is not supported directly on Windows; use WSL"),
		)
	}
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		return newDemoFailure(
			demoErrorAuthentication,
			"Authentication failed.",
			fmt.Errorf("authentication failed; run `qovery auth` first: %w", err),
		)
	}
	organizationID, _, err := utils.CurrentOrganization(true)
	if err != nil {
		return newDemoFailure(
			demoErrorOrganization,
			"The current organization could not be resolved.",
			fmt.Errorf("cannot resolve the current organization: %w", err),
		)
	}
	if err := validateDemoClusterName(demoClusterName); err != nil {
		return newDemoFailure(demoErrorInvalidClusterName, "The cluster name is invalid.", err)
	}
	debugLog, debugLogsPath, err := openDemoDebugLog()
	if err != nil {
		return err
	}
	defer func() { _ = debugLog.Close() }()
	_, _ = fmt.Fprintf(debugLog, "Diagnostic ID: %s\n", diagnosticID)

	terminalOutput := cmd.OutOrStdout()
	runner := &demoExecRunner{
		out:   terminalOutput,
		log:   debugLog,
		debug: demoDebug,
	}
	orchestrator := demoOrchestrator{
		api: &demoQoveryAPI{client: utils.GetQoveryClient(tokenType, token)},
		local: &demoLocalCommands{
			runner:      runner,
			goos:        runtime.GOOS,
			tempDir:     filepath.Dir(debugLogsPath),
			kubeContext: demoKubeContext(demoClusterName),
		},
		clock:    demoSystemClock{},
		out:      io.MultiWriter(terminalOutput, debugLog),
		progress: progress,
	}
	err = orchestrator.Up(ctx, demoConfig{
		OrganizationID:  string(organizationID),
		ClusterName:     demoClusterName,
		CPUArchitecture: detectArchitecture(),
	})
	if err != nil {
		_, _ = fmt.Fprintf(debugLog, "\nERROR: %v\n", err)
		_ = debugLog.Sync()
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %w", errDemoInterrupted, err)
		}
		if refreshedType, refreshedToken, refreshErr := utils.GetAccessToken(false); refreshErr == nil {
			tokenType, token = refreshedType, refreshedToken
		}
		uploadErrorLogs(tokenType, token, organizationID, demoClusterName, debugLogsPath)
		return err
	}
	return nil
}

func (a *demoAttempt) capture(
	cmd *cobra.Command,
	event string,
	execution utils.CommandExecutionProperties,
) {
	execution.AttemptID = a.id
	execution.WorkflowType = demoWorkflowType
	execution.Implementation = demoImplementation
	execution.ClusterID = a.clusterID
	if execution.Result != "" && execution.DurationMillis <= 0 {
		execution.DurationMillis = max(time.Since(a.startedAt).Milliseconds(), 1)
	}
	utils.CaptureCommandExecution(cmd, event, execution)
}

func (t *demoAttemptTelemetry) ClusterResolved(clusterID string) {
	t.attempt.clusterID = clusterID
}

func (t *demoAttemptTelemetry) PhaseFinished(execution demoPhaseExecution) {
	if execution.Result != demoPhaseResultSucceeded {
		t.attempt.failedPhase = execution.Phase
		t.attempt.canceled = execution.Result == demoPhaseResultCanceled
	}
	properties := utils.CommandExecutionProperties{
		Phase:          execution.Phase,
		Result:         execution.Result,
		DurationMillis: max(execution.Duration.Milliseconds(), 1),
	}
	if execution.Result == demoPhaseResultFailed {
		properties.ErrorCode = execution.ErrorCode
		properties.SafeErrorMessage = execution.SafeErrorMessage
	}
	t.attempt.capture(t.command, utils.PhaseFinishedEventName, properties)
}

func (a *demoAttempt) printFailureSummary(cmd *cobra.Command, safeMessage string) {
	phase := strings.ReplaceAll(a.failedPhase, "_", " ")
	if phase == "" {
		phase = "an unknown phase"
	}
	_, _ = fmt.Fprintf(
		cmd.ErrOrStderr(),
		"\nDemo installation failed during %s.\n%s\nDiagnostic ID: %s\n",
		phase,
		safeMessage,
		a.id,
	)
}

func newDemoAttempt() *demoAttempt {
	return &demoAttempt{
		id:        uuid.Must(uuid.NewV7()).String(),
		startedAt: time.Now(),
	}
}

func init() {
	demoClusterName = "local-demo-" + demoSafeUsername()
	demoUpCmd.Flags().StringVarP(&demoClusterName, "cluster-name", "c", demoClusterName, "The name of the local demo cluster")
	demoUpCmd.Flags().BoolVar(&demoDebug, "debug", false, "Enable debug mode")
	demoCmd.AddCommand(demoUpCmd)
}

func openDemoDebugLog() (*os.File, string, error) {
	directory := filepath.Join(os.TempDir(), "qovery-demo")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, "", fmt.Errorf("cannot create demo log directory: %w", err)
	}
	logPath := filepath.Join(directory, "qovery-demo.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, "", fmt.Errorf("cannot create demo log file: %w", err)
	}
	return file, logPath, nil
}

func demoSafeUsername() string {
	current, err := user.Current()
	if err != nil {
		return "qovery"
	}
	value := strings.ToLower(current.Username)
	value = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "qovery"
	}
	return value
}

func validateDemoClusterName(name string) error {
	if !regexp.MustCompile(`^[a-zA-Z][-a-zA-Z0-9]*[a-zA-Z0-9]$`).MatchString(name) {
		return fmt.Errorf("cluster name must start with a letter, end with a letter or digit, and contain only letters, digits, or hyphens: got %q", name)
	}
	return nil
}

type demoQoveryAPI struct {
	client *qovery.APIClient
}

// callDemoAPI refreshes the access token once on a 401: an installation outlives its 15-minute lifetime.
// It never falls back to a browser login, which would block the installation.
func callDemoAPI[T any](a *demoQoveryAPI, call func(*qovery.APIClient) (*T, *http.Response, error)) (*T, *http.Response, error) {
	result, response, err := call(a.client)
	if response != nil && response.StatusCode == http.StatusUnauthorized {
		tokenType, token, tokenErr := utils.GetAccessToken(false)
		if tokenErr != nil {
			return nil, response, fmt.Errorf("the Qovery session expired and cannot be refreshed; run `qovery auth` and retry: %w", tokenErr)
		}
		a.client = utils.GetQoveryClient(tokenType, token)
		result, response, err = call(a.client)
	}
	if err == nil && result == nil {
		return nil, response, errors.New("qovery API returned an empty response")
	}
	return result, response, err
}

func callDemoAPIWithoutBody(a *demoQoveryAPI, call func(*qovery.APIClient) (*http.Response, error)) (*http.Response, error) {
	_, response, err := callDemoAPI(a, func(client *qovery.APIClient) (*struct{}, *http.Response, error) {
		response, err := call(client)
		return &struct{}{}, response, err
	})
	return response, err
}

func (a *demoQoveryAPI) EnsureOnPremiseCredentials(ctx context.Context, organizationID string) (demoCredential, error) {
	list, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterCredentialsResponseList, *http.Response, error) {
		return client.CloudProviderCredentialsAPI.ListOnPremiseCredentials(ctx, organizationID).Execute()
	})
	if err != nil {
		return demoCredential{}, err
	}
	if results := list.GetResults(); len(results) > 0 {
		return demoCredentialFromQovery(results[0])
	}
	created, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterCredentials, *http.Response, error) {
		return client.CloudProviderCredentialsAPI.CreateOnPremiseCredentials(ctx, organizationID).
			OnPremiseCredentialsRequest(qovery.OnPremiseCredentialsRequest{Name: "on-premise"}).
			Execute()
	})
	if err != nil {
		return demoCredential{}, err
	}
	return demoCredentialFromQovery(*created)
}

func demoCredentialFromQovery(value qovery.ClusterCredentials) (demoCredential, error) {
	generic, ok := value.GetActualInstance().(*qovery.GenericClusterCredentials)
	if !ok || generic == nil || generic.Id == "" {
		return demoCredential{}, errors.New("qovery returned invalid On-Premise credentials")
	}
	return demoCredential{ID: generic.Id, Name: generic.Name}, nil
}

func (a *demoQoveryAPI) FindCluster(ctx context.Context, organizationID string, name string) (string, bool, error) {
	clusters, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterResponseList, *http.Response, error) {
		return client.ClustersAPI.ListOrganizationCluster(ctx, organizationID).Execute()
	})
	if err != nil {
		return "", false, err
	}
	return findDemoCluster(clusters.GetResults(), name)
}

var errDemoInterrupted = errors.New("demo installation interrupted")

var errNotADemoCluster = errors.New("is not a demo cluster; qovery demo only manages demo clusters")

func findDemoCluster(clusters []qovery.Cluster, name string) (string, bool, error) {
	cluster := utils.FindByClusterName(clusters, name)
	if cluster == nil {
		return "", false, nil
	}
	if !cluster.GetIsDemo() {
		return "", false, fmt.Errorf("cluster %q in this organization %w", name, errNotADemoCluster)
	}
	return cluster.Id, true, nil
}

func newDemoClusterRequest(name string, credential demoCredential) qovery.ClusterRequest {
	production := false
	kubernetes := qovery.KUBERNETESENUM_SELF_MANAGED
	provider := qovery.CLOUDPROVIDERENUM_ON_PREMISE
	region := "unknown"
	isDemo := true
	return qovery.ClusterRequest{
		Name:          name,
		Region:        "on-premise",
		CloudProvider: qovery.CLOUDVENDORENUM_ON_PREMISE,
		Kubernetes:    &kubernetes,
		Production:    &production,
		CloudProviderCredentials: &qovery.ClusterCloudProviderInfoRequest{
			CloudProvider: &provider,
			Credentials: &qovery.ClusterCloudProviderInfoCredentials{
				Id:   &credential.ID,
				Name: &credential.Name,
			},
			Region: &region,
		},
		Features: []qovery.ClusterRequestFeaturesInner{},
		IsDemo:   &isDemo,
	}
}

func (a *demoQoveryAPI) CreateCluster(ctx context.Context, organizationID string, name string, credential demoCredential) (string, error) {
	cluster, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.Cluster, *http.Response, error) {
		return client.ClustersAPI.CreateCluster(ctx, organizationID).
			ClusterRequest(newDemoClusterRequest(name, credential)).
			Execute()
	})
	if err != nil {
		return "", err
	}
	return cluster.Id, nil
}

func (a *demoQoveryAPI) ConfigureOperator(ctx context.Context, organizationID string, clusterID string, cpuArchitecture string) error {
	architecture, err := normalizeDemoCPUArchitecture(cpuArchitecture)
	if err != nil {
		return err
	}
	demoConfiguration, err := a.demoPlatformConfiguration(ctx, organizationID)
	if err != nil {
		return err
	}
	existingConfiguration, response, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterPlatformConfigurationResponse, *http.Response, error) {
		return client.PlatformConfigurationAPI.GetClusterPlatformConfiguration(ctx, clusterID).Execute()
	})
	if err != nil {
		if response == nil || response.StatusCode != http.StatusNotFound {
			return fmt.Errorf("cannot read the cluster platform configuration: %w", err)
		}
		existingConfiguration = nil
	}
	configuration := selectDemoPlatformConfiguration(existingConfiguration, demoConfiguration)
	if demoOperatorIsAlreadyConfigured(existingConfiguration, configuration, architecture) {
		return nil
	}
	request, err := newDemoOperatorConfigurationRequest(configuration, architecture)
	if err != nil {
		return err
	}
	if err := ensureDemoRequestHasNoRedactedValues(request); err != nil {
		return err
	}
	_, response, err = callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterPlatformConfigurationResponse, *http.Response, error) {
		return client.PlatformConfigurationAPI.UpdateClusterPlatformConfiguration(ctx, clusterID).
			ClusterPlatformConfigurationRequest(request).
			Execute()
	})
	if err != nil {
		return demoPlatformConfigurationSaveError(response, err)
	}
	return nil
}

func (a *demoQoveryAPI) demoPlatformConfiguration(ctx context.Context, organizationID string) (*qovery.ClusterPlatformConfigurationResponse, error) {
	catalog, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.PlatformTemplateCatalogResponse, *http.Response, error) {
		return client.PlatformConfigurationAPI.ListPlatformTemplates(ctx, organizationID).
			ClusterMode(qovery.PLATFORMCLUSTERMODE_CUSTOMER_MANAGED).
			CloudProvider(qovery.PLATFORMCLOUDVENDOR_UNKNOWN).
			Execute()
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list the platform templates: %w", err)
	}
	return newDemoPlatformConfiguration(catalog)
}

func newDemoPlatformConfiguration(catalog *qovery.PlatformTemplateCatalogResponse) (*qovery.ClusterPlatformConfigurationResponse, error) {
	if catalog == nil {
		return nil, fmt.Errorf("qovery returned no %q platform template for the local demo", demoPlatformTemplateKey)
	}
	var template *qovery.PlatformTemplateSummaryResponse
	for i := range catalog.Results {
		candidate := &catalog.Results[i]
		if candidate.Key == demoPlatformTemplateKey && strings.TrimSpace(candidate.Version) != "" {
			template = candidate
			break
		}
	}
	if template == nil {
		return nil, fmt.Errorf("qovery returned no %q platform template for the local demo", demoPlatformTemplateKey)
	}
	layerSelections := make(map[string]bool)
	for _, layer := range template.Layers {
		if !layer.Mandatory {
			layerSelections[layer.Key] = layer.EnabledByDefault
		}
	}
	platform := qovery.NewPlatformSelection(template.Key, template.Version)
	platform.SetLayerSelections(layerSelections)
	platform.SetManagedConfig(map[string]map[string]interface{}{})
	return &qovery.ClusterPlatformConfigurationResponse{
		Platform:      *platform,
		ClusterInputs: map[string]map[string]string{},
	}, nil
}

func selectDemoPlatformConfiguration(
	existingConfiguration *qovery.ClusterPlatformConfigurationResponse,
	demoConfiguration *qovery.ClusterPlatformConfigurationResponse,
) *qovery.ClusterPlatformConfigurationResponse {
	if existingConfiguration != nil && demoConfiguration != nil &&
		existingConfiguration.Platform.TemplateKey == demoConfiguration.Platform.TemplateKey &&
		existingConfiguration.Platform.TemplateVersion == demoConfiguration.Platform.TemplateVersion {
		return existingConfiguration
	}
	return demoConfiguration
}

const (
	demoOperatorComponentKey  = "qovery-operator"
	demoCPUArchitecturesField = "cpuArchitectures"
	demoRedactedValue         = "<redacted>"
)

func normalizeDemoCPUArchitecture(cpuArchitecture string) (string, error) {
	architecture := strings.ToUpper(strings.TrimSpace(cpuArchitecture))
	if architecture != "AMD64" && architecture != "ARM64" {
		return "", fmt.Errorf("unsupported local CPU architecture %q", cpuArchitecture)
	}
	return architecture, nil
}

func demoOperatorIsAlreadyConfigured(
	existingConfiguration *qovery.ClusterPlatformConfigurationResponse,
	configuration *qovery.ClusterPlatformConfigurationResponse,
	architecture string,
) bool {
	if existingConfiguration == nil || configuration != existingConfiguration {
		return false
	}
	configured, _ := existingConfiguration.Platform.GetManagedConfig()[demoOperatorComponentKey][demoCPUArchitecturesField].(string)
	return configured == architecture
}

func ensureDemoRequestHasNoRedactedValues(request qovery.ClusterPlatformConfigurationRequest) error {
	var fields []string
	for component, values := range request.Platform.GetManagedConfig() {
		for key, value := range values {
			if text, ok := value.(string); ok && text == demoRedactedValue {
				fields = append(fields, component+"."+key)
			}
		}
	}
	if len(fields) == 0 {
		return nil
	}
	sort.Strings(fields)
	return fmt.Errorf(
		"qovery returned redacted values for %s; qovery demo sends the whole configuration back and would store the redaction marker instead, so nothing was saved. Retry later; if it persists, these fields are sensitive and qovery demo cannot keep them: clear them, or recreate the cluster with `qovery demo destroy --delete-qovery-config`",
		strings.Join(fields, ", "),
	)
}

func demoPlatformConfigurationSaveError(response *http.Response, err error) error {
	var apiError *qovery.GenericOpenAPIError
	if response != nil && response.StatusCode == http.StatusConflict && errors.As(err, &apiError) {
		if message := strings.TrimSpace(string(apiError.Body())); message != "" {
			return fmt.Errorf("cannot save the cluster platform configuration: %w: %s", err, message)
		}
	}
	return fmt.Errorf("cannot save the cluster platform configuration: %w", err)
}

func newDemoOperatorConfigurationRequest(configuration *qovery.ClusterPlatformConfigurationResponse, architecture string) (qovery.ClusterPlatformConfigurationRequest, error) {
	if configuration == nil || strings.TrimSpace(configuration.Platform.TemplateKey) == "" || strings.TrimSpace(configuration.Platform.TemplateVersion) == "" {
		return qovery.ClusterPlatformConfigurationRequest{}, errors.New("qovery returned an invalid platform configuration")
	}

	existingManagedConfig := configuration.Platform.GetManagedConfig()
	managedConfig := make(map[string]map[string]interface{}, len(existingManagedConfig)+1)
	for component, values := range existingManagedConfig {
		managedConfig[component] = make(map[string]interface{}, len(values))
		for key, value := range values {
			managedConfig[component][key] = value
		}
	}
	operatorConfig := managedConfig[demoOperatorComponentKey]
	if operatorConfig == nil {
		operatorConfig = map[string]interface{}{}
	}
	operatorConfig[demoCPUArchitecturesField] = architecture
	managedConfig[demoOperatorComponentKey] = operatorConfig

	layerSelections := configuration.Platform.GetLayerSelections()
	if layerSelections == nil {
		layerSelections = map[string]bool{}
	}
	clusterInputs := configuration.ClusterInputs
	if clusterInputs == nil {
		clusterInputs = map[string]map[string]string{}
	}

	platform := qovery.NewPlatformSelection(configuration.Platform.TemplateKey, configuration.Platform.TemplateVersion)
	platform.SetLayerSelections(layerSelections)
	platform.SetManagedConfig(managedConfig)
	return *qovery.NewClusterPlatformConfigurationRequest(*platform, clusterInputs), nil
}

func (a *demoQoveryAPI) GetOperatorBootstrap(ctx context.Context, organizationID string, clusterID string) (demoBootstrap, error) {
	bootstrap, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterOperatorBootstrapResponse, *http.Response, error) {
		return client.ClusterOperatorAPI.GetClusterOperatorBootstrap(ctx, organizationID, clusterID).Execute()
	})
	if err != nil {
		return demoBootstrap{}, err
	}
	return demoBootstrap{
		ReleaseName:    bootstrap.ReleaseName,
		ChartReference: bootstrap.ChartReference,
		ChartVersion:   bootstrap.ChartVersion,
		Namespace:      bootstrap.Namespace,
		ValuesYAML:     bootstrap.ValuesYaml,
	}, nil
}

func (a *demoQoveryAPI) AttachOperator(ctx context.Context, organizationID string, clusterID string) error {
	_, err := callDemoAPIWithoutBody(a, func(client *qovery.APIClient) (*http.Response, error) {
		return client.ClusterOperatorAPI.AttachClusterOperator(ctx, organizationID, clusterID).Execute()
	})
	return err
}

func (a *demoQoveryAPI) GetOperatorStatus(ctx context.Context, organizationID string, clusterID string) (demoOperatorStatus, error) {
	status, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterOperatorStatusResponse, *http.Response, error) {
		return client.ClusterOperatorAPI.GetClusterOperatorStatus(ctx, organizationID, clusterID).Execute()
	})
	if err != nil {
		return demoOperatorStatus{}, err
	}
	return demoOperatorStatus{
		Connected:     status.OperatorConnected,
		LastHeartbeat: status.LastHeartbeat.Get(),
	}, nil
}

func (a *demoQoveryAPI) DeployCluster(ctx context.Context, organizationID string, clusterID string) (string, error) {
	// q-core answers QUEUED here, never the previous terminal status (ClusterApiController.deployCluster).
	status, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterStatus, *http.Response, error) {
		return client.ClustersAPI.DeployCluster(ctx, organizationID, clusterID).Execute()
	})
	if err != nil {
		return "", err
	}
	return string(status.Status), nil
}

func (a *demoQoveryAPI) GetClusterStatus(ctx context.Context, organizationID string, clusterID string) (string, error) {
	status, _, err := callDemoAPI(a, func(client *qovery.APIClient) (*qovery.ClusterStatus, *http.Response, error) {
		return client.ClustersAPI.GetClusterStatus(ctx, organizationID, clusterID).Execute()
	})
	if err != nil {
		return "", err
	}
	return string(status.Status), nil
}

type demoCommandRunner interface {
	LookPath(string) error
	Run(context.Context, string, ...string) ([]byte, error)
	RunQuiet(context.Context, string, ...string) ([]byte, error)
	RunUnlogged(context.Context, string, ...string) ([]byte, error)
}

type demoExecRunner struct {
	out   io.Writer
	log   io.Writer
	debug bool
}

func (r *demoExecRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (r *demoExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.run(ctx, true, name, args...)
}

func (r *demoExecRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.run(ctx, false, name, args...)
}

// RunUnlogged keeps the output out of the debug log, which is uploaded on failure.
func (r *demoExecRunner) RunUnlogged(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.execute(ctx, false, false, name, args...)
}

func (r *demoExecRunner) run(ctx context.Context, visible bool, name string, args ...string) ([]byte, error) {
	return r.execute(ctx, visible, true, name, args...)
}

func (r *demoExecRunner) execute(ctx context.Context, visible bool, logOutput bool, name string, args ...string) ([]byte, error) {
	commandLine := strings.Join(append([]string{name}, args...), " ")
	if r.log != nil {
		_, _ = fmt.Fprintf(r.log, "$ %s\n", commandLine)
	}
	showCommand := visible || r.debug
	streamOutput := visible || (r.debug && logOutput)
	if showCommand && r.out != nil {
		_, _ = fmt.Fprintf(r.out, "$ %s\n", commandLine)
	}

	var output bytes.Buffer
	writers := []io.Writer{&output}
	if r.log != nil && logOutput {
		writers = append(writers, r.log)
	}
	if streamOutput && r.out != nil {
		writers = append(writers, r.out)
	}
	command := exec.CommandContext(ctx, name, args...)
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = demoCommandInterruptGrace
	command.Stdout = io.MultiWriter(writers...)
	command.Stderr = command.Stdout
	err := command.Run()
	if err != nil && !streamOutput && r.out != nil {
		if !showCommand {
			_, _ = fmt.Fprintf(r.out, "$ %s\n", commandLine)
		}
		_, _ = r.out.Write(output.Bytes())
	}
	return output.Bytes(), err
}

type demoLocalCommands struct {
	runner      demoCommandRunner
	goos        string
	tempDir     string
	kubeContext string
}

func demoKubeContext(clusterName string) string {
	return "k3d-" + clusterName
}

func (l *demoLocalCommands) CheckDependencies(ctx context.Context) error {
	var missing []string
	for _, dependency := range []string{"docker", "k3d", "helm", "kubectl"} {
		if err := l.runner.LookPath(dependency); err != nil {
			missing = append(missing, dependency)
		}
	}
	if len(missing) > 0 {
		return missingDemoDependenciesError(l.goos, missing)
	}
	if _, err := l.runner.RunQuiet(ctx, "docker", "info"); err != nil {
		return newDemoFailure(demoErrorDockerNotRunning, "Docker is not running.", errors.New("docker is not running"))
	}
	return nil
}

func (l *demoLocalCommands) EnsureK3dCluster(ctx context.Context, name string) error {
	output, err := l.runner.RunUnlogged(ctx, "k3d", "cluster", "list", "--output", "json")
	if err != nil {
		return commandFailed("k3d cluster list", err)
	}
	var clusters []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &clusters); err != nil {
		return errors.New("k3d returned an invalid cluster list")
	}
	for _, cluster := range clusters {
		if cluster.Name == name {
			if _, err := l.runner.Run(ctx, "k3d", "cluster", "start", name); err != nil {
				return commandFailed("k3d cluster start", err)
			}
			return l.ensureDemoRegistry(ctx, name)
		}
	}
	if err := l.ensureDemoSubnetIsFree(ctx, name); err != nil {
		return err
	}
	args := []string{
		"cluster", "create", name,
		"--image", demoK3sImage,
		"--subnet", demoSubnet,
		"--k3s-arg", "--node-ip=" + demoNodeIP + "@server:0",
		"--k3s-arg", "--disable=traefik@server:*",
		"--registry-create", demoRegistry,
		"--port", "80:80@loadbalancer",
		"--port", "443:443@loadbalancer",
	}
	if _, err := l.runner.Run(ctx, "k3d", args...); err != nil {
		return commandFailed("k3d cluster create", err)
	}
	return l.ensureDemoRegistry(ctx, name)
}

func (l *demoLocalCommands) ensureDemoSubnetIsFree(ctx context.Context, clusterName string) error {
	ids, err := l.runner.RunQuiet(ctx, "docker", "network", "ls", "--quiet")
	if err != nil || len(strings.Fields(string(ids))) == 0 {
		return nil
	}
	output, err := l.runner.RunUnlogged(ctx, "docker", append([]string{"network", "inspect"}, strings.Fields(string(ids))...)...)
	if err != nil {
		return nil
	}
	var networks []struct {
		Name string `json:"Name"`
		IPAM struct {
			Config []struct {
				Subnet string `json:"Subnet"`
			} `json:"Config"`
		} `json:"IPAM"`
	}
	if json.Unmarshal(output, &networks) != nil {
		return nil
	}
	demoPrefix := netip.MustParsePrefix(demoSubnet)
	for _, network := range networks {
		if network.Name == "k3d-"+clusterName {
			continue
		}
		for _, config := range network.IPAM.Config {
			prefix, err := netip.ParsePrefix(config.Subnet)
			if err == nil && prefix.Overlaps(demoPrefix) {
				return demoSubnetInUseError(network.Name)
			}
		}
	}
	return nil
}

func (l *demoLocalCommands) ensureDemoRegistry(ctx context.Context, clusterName string) error {
	containerName, found, err := l.findDemoRegistry(ctx)
	if err != nil {
		return err
	}
	if found {
		return l.ensureDemoRegistryAlias(ctx, clusterName, containerName)
	}
	if _, err := l.runner.Run(
		ctx,
		"k3d", "registry", "create", demoRegistry,
		"--default-network", "k3d-"+clusterName,
		"--no-help",
	); err != nil {
		return commandFailed("k3d registry create", err)
	}
	containerName, found, err = l.findDemoRegistry(ctx)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("k3d did not expose the created demo registry")
	}
	return l.ensureDemoRegistryAlias(ctx, clusterName, containerName)
}

func (l *demoLocalCommands) findDemoRegistry(ctx context.Context) (string, bool, error) {
	output, err := l.runner.RunQuiet(ctx, "k3d", "registry", "list", "--output", "json")
	if err != nil {
		return "", false, commandFailed("k3d registry list", err)
	}
	var registries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &registries); err != nil {
		return "", false, errors.New("k3d returned an invalid registry list")
	}
	for _, registry := range registries {
		if registry.Name == demoRegistry || registry.Name == "k3d-"+demoRegistry {
			return registry.Name, true, nil
		}
	}
	return "", false, nil
}

func (l *demoLocalCommands) ensureDemoRegistryAlias(ctx context.Context, clusterName string, containerName string) error {
	networkName := "k3d-" + clusterName
	output, err := l.runner.RunQuiet(ctx, "docker", "inspect", containerName)
	if err != nil {
		return commandFailed("docker inspect demo registry", err)
	}
	var containers []struct {
		NetworkSettings struct {
			Networks map[string]struct {
				Aliases  []string `json:"Aliases"`
				DNSNames []string `json:"DNSNames"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(output, &containers); err != nil || len(containers) != 1 {
		return errors.New("docker returned invalid demo registry details")
	}
	network, connected := containers[0].NetworkSettings.Networks[networkName]
	if connected && (containsString(network.Aliases, demoRegistry) || containsString(network.DNSNames, demoRegistry)) {
		return nil
	}
	if connected {
		if _, err := l.runner.Run(ctx, "docker", "network", "disconnect", networkName, containerName); err != nil {
			return commandFailed("docker network disconnect demo registry", err)
		}
	}
	if _, err := l.runner.Run(
		ctx,
		"docker", "network", "connect", "--alias", demoRegistry, networkName, containerName,
	); err != nil {
		return commandFailed("docker network connect demo registry", err)
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func loopbackHasAddress(output []byte, ip string) bool {
	fields := strings.Fields(string(output))
	for i, field := range fields {
		if field == "inet" && i+1 < len(fields) && strings.SplitN(fields[i+1], "/", 2)[0] == ip {
			return true
		}
	}
	return false
}

func (l *demoLocalCommands) EnsureLoopback(ctx context.Context) error {
	switch l.goos {
	case "darwin":
		output, err := l.runner.RunQuiet(ctx, "ifconfig", "lo0")
		if err != nil {
			return commandFailed("macOS loopback inspection", err)
		}
		if loopbackHasAddress(output, demoNodeIP) {
			return nil
		}
		if _, err := l.runner.Run(ctx, "sudo", "ifconfig", "lo0", "alias", demoNodeIP+"/32", "up"); err != nil {
			return commandFailed("macOS loopback configuration", err)
		}
	case "linux":
		version, err := os.ReadFile("/proc/version")
		if err == nil && strings.Contains(strings.ToLower(string(version)), "microsoft") {
			output, err := l.runner.Run(ctx, "sudo", "ip", "addr", "add", demoNodeIP+"/32", "dev", "lo")
			if err != nil && !strings.Contains(strings.ToLower(string(output)), "exists") {
				return commandFailed("WSL loopback configuration", err)
			}
			powershell := "powershell.exe"
			if err := l.runner.LookPath(powershell); err != nil {
				powershell = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
			}
			command := fmt.Sprintf("Start-Process netsh -Verb RunAs -Wait -ArgumentList \"interface ipv4 add address name='Loopback Pseudo-Interface 1' address=%s mask=255.255.255.255 skipassource=true\"", demoNodeIP)
			output, err = l.runner.Run(ctx, powershell, "-NoProfile", "-Command", command)
			if err != nil && !strings.Contains(strings.ToLower(string(output)), "exists") {
				return commandFailed("Windows loopback configuration", err)
			}
		}
	}
	return nil
}

func (l *demoLocalCommands) LegacyQoveryReleaseExists(ctx context.Context) (bool, error) {
	// Helm 4 removed `helm list --all`; listing every status explicitly works on Helm 3 and 4.
	output, err := l.runner.RunQuiet(ctx, "helm", "list", "--kube-context", l.kubeContext, "--namespace", "qovery", "--deployed", "--failed", "--pending", "--superseded", "--uninstalling", "--output", "json", "--filter", "^qovery$")
	if err != nil {
		return false, commandFailed("helm list", err)
	}
	var releases []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &releases); err != nil {
		return false, errors.New("helm returned an invalid release list")
	}
	for _, release := range releases {
		if release.Name == "qovery" {
			return true, nil
		}
	}
	return false, nil
}

func buildDemoHelmArgs(bootstrap demoBootstrap, valuesPath string, kubeContext string) ([]string, error) {
	if strings.TrimSpace(bootstrap.ReleaseName) == "" || strings.TrimSpace(bootstrap.ChartReference) == "" || strings.TrimSpace(bootstrap.ChartVersion) == "" || strings.TrimSpace(bootstrap.Namespace) == "" {
		return nil, errors.New("operator bootstrap is missing structured Helm fields")
	}
	if path.Base(strings.TrimSuffix(bootstrap.ChartReference, "/")) == "qovery" {
		return nil, errors.New("refusing to install the legacy Qovery umbrella chart")
	}
	return []string{
		"upgrade", "--install",
		"--kube-context", kubeContext,
		bootstrap.ReleaseName,
		bootstrap.ChartReference,
		"--version", bootstrap.ChartVersion,
		"--namespace", bootstrap.Namespace,
		"--values", valuesPath,
		"--create-namespace",
		"--atomic",
		"--wait",
		"--timeout", "15m",
	}, nil
}

func (l *demoLocalCommands) InstallOperator(ctx context.Context, bootstrap demoBootstrap) error {
	file, err := os.CreateTemp(l.tempDir, "qovery-demo-operator-values-*.yaml")
	if err != nil {
		return errors.New("cannot create protected temporary Operator values file")
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return errors.New("cannot protect temporary Operator values file")
	}
	if _, err := file.WriteString(bootstrap.ValuesYAML); err != nil {
		_ = file.Close()
		return errors.New("cannot write temporary Operator values file")
	}
	if err := file.Close(); err != nil {
		return errors.New("cannot close temporary Operator values file")
	}
	args, err := buildDemoHelmArgs(bootstrap, path, l.kubeContext)
	if err != nil {
		return err
	}
	if _, err := l.runner.Run(ctx, "helm", args...); err != nil {
		return commandFailed("helm upgrade --install", err)
	}
	return nil
}

func (l *demoLocalCommands) ValidateWorkloads(ctx context.Context, namespace string) error {
	output, err := l.runner.RunQuiet(ctx, "kubectl", "--context", l.kubeContext, "--namespace", namespace, "get", "deployments", "--output", "json")
	if err != nil {
		return commandFailed("kubectl get deployments", err)
	}
	var deployments struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &deployments); err != nil {
		return errors.New("kubectl returned an invalid deployment list")
	}
	operatorFound := false
	for _, deployment := range deployments.Items {
		switch deployment.Metadata.Name {
		case "qovery-operator":
			operatorFound = true
		case "qovery-engine":
			return fmt.Errorf("unexpected permanent Deployment qovery-engine exists in namespace %s", namespace)
		}
	}
	if !operatorFound {
		return errors.New("deployment qovery-operator was not found after installation")
	}
	return nil
}

func commandFailed(operation string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s failed with exit code %d", operation, exitErr.ExitCode())
	}
	return fmt.Errorf("%s failed", operation)
}

type demoSystemClock struct{}

func (demoSystemClock) Now() time.Time { return time.Now() }

func (demoSystemClock) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Only needed due to MacOs when rosetta (x86_64 emulation on ARM64) is turned on.
// otherwise GOARCH runtime variable is enough to detect the correct arch
func detectArchitecture() string {
	if runtime.GOOS != "darwin" {
		return strings.ToUpper(runtime.GOARCH)
	}

	return strings.ToUpper(rosetta.NativeArch())
}

func uploadErrorLogs(tokenType utils.AccessTokenType, token utils.AccessToken, organization utils.Id, clusterName string, debugLogsPath string) {
	type Payload struct {
		Organization string    `json:"organization"`
		ClusterName  string    `json:"cluster_name"`
		Content      string    `json:"content"`
		Os           string    `json:"os"`
		CpuArch      string    `json:"cpu_arch"`
		CliVersion   string    `json:"cli_version"`
		Timestamp    time.Time `json:"timestamp"`
	}

	content, _ := os.ReadFile(debugLogsPath)
	payload, _ := json.Marshal(Payload{
		Organization: string(organization),
		ClusterName:  clusterName,
		Content:      string(content),
		Os:           runtime.GOOS,
		CpuArch:      detectArchitecture(),
		CliVersion:   utils.Version,
		Timestamp:    time.Now(),
	})
	client := utils.GetQoveryClient(tokenType, token)
	query := url.Values{"organization": {string(organization)}, "clusterName": {clusterName}}
	endpoint := fmt.Sprintf("%s/admin/demoDebugLog?%s", client.GetConfig().Servers[0].URL, query.Encode())

	err := postDemoDebugLog(&http.Client{Timeout: 30 * time.Second}, endpoint, utils.GetAuthorizationHeaderValue(tokenType, token), payload, demoDebugLogRetryDelay)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("error uploading debug logs: %w", err))
		if isDemoDebugLogAuthFailure(err) {
			utils.PrintlnInfo("May be caused by a wrong context set, please set it again: `qovery context set`")
		}
	}
}

const demoDebugLogRetryDelay = 2 * time.Second

type demoDebugLogUploadError struct {
	status     string
	statusCode int
	body       []byte
}

func (e *demoDebugLogUploadError) Error() string {
	return fmt.Sprintf("%s %s", e.status, e.body)
}

func postDemoDebugLog(httpClient *http.Client, endpoint string, authorization string, payload []byte, retryDelay time.Duration) error {
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			time.Sleep(retryDelay)
		}
		err = postDemoDebugLogOnce(httpClient, endpoint, authorization, payload)
		var uploadErr *demoDebugLogUploadError
		if err == nil || (errors.As(err, &uploadErr) && uploadErr.statusCode < http.StatusInternalServerError) {
			return err
		}
	}
	return err
}

func postDemoDebugLogOnce(httpClient *http.Client, endpoint string, authorization string, payload []byte) error {
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(response.Body)
	return &demoDebugLogUploadError{status: response.Status, statusCode: response.StatusCode, body: body}
}

func isDemoDebugLogAuthFailure(err error) bool {
	var uploadErr *demoDebugLogUploadError
	return errors.As(err, &uploadErr) && (uploadErr.statusCode == http.StatusUnauthorized || uploadErr.statusCode == http.StatusForbidden)
}
