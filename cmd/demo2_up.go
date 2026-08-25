package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

const (
	demo2K3sImage            = "rancher/k3s:v1.33.5-k3s1"
	demo2Subnet              = "172.42.0.0/16"
	demo2NodeIP              = "172.42.0.3"
	demo2Registry            = "qovery-registry.lan"
	demo2PlatformTemplateKey = "qovery-demo-v0"
)

var (
	demo2ClusterName string
	demo2Debug       bool
)

var demo2UpCmd = &cobra.Command{
	Use:   "up",
	Short: "Create an experimental local cluster using Qovery Operator and Engine V2",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.Capture(cmd)
		if runtime.GOOS == "windows" {
			return errors.New("qovery demo2 is not supported directly on Windows; use WSL")
		}
		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			return fmt.Errorf("authentication failed; run `qovery auth` first: %w", err)
		}
		organizationID, _, err := utils.CurrentOrganization(true)
		if err != nil {
			return fmt.Errorf("cannot resolve the current organization: %w", err)
		}
		if err := validateDemo2ClusterName(demo2ClusterName); err != nil {
			return err
		}
		debugLog, debugLogsPath, err := openDemo2DebugLog()
		if err != nil {
			return err
		}
		defer func() { _ = debugLog.Close() }()

		terminalOutput := cmd.OutOrStdout()
		runner := &demo2ExecRunner{
			out:   terminalOutput,
			log:   debugLog,
			debug: demo2Debug,
		}
		orchestrator := demo2Orchestrator{
			api:   &demo2QoveryAPI{client: utils.GetQoveryClient(tokenType, token)},
			local: &demo2LocalCommands{runner: runner, goos: runtime.GOOS},
			clock: demo2SystemClock{},
			out:   io.MultiWriter(terminalOutput, debugLog),
		}
		err = orchestrator.Up(cmd.Context(), demo2Config{
			OrganizationID:  string(organizationID),
			ClusterName:     demo2ClusterName,
			CPUArchitecture: detectArchitecture(),
		})
		if err != nil {
			_, _ = fmt.Fprintf(debugLog, "\nERROR: %v\n", err)
			_ = debugLog.Sync()
			uploadErrorLogs(tokenType, token, organizationID, demo2ClusterName, debugLogsPath)
			utils.CaptureError(cmd, "qovery demo2 up", err.Error())
			return err
		}
		utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
		return nil
	},
}

func init() {
	demo2ClusterName = "local-demo2-" + demo2SafeUsername()
	demo2UpCmd.Flags().StringVarP(&demo2ClusterName, "cluster-name", "c", demo2ClusterName, "The name of the experimental local cluster")
	demo2UpCmd.Flags().BoolVar(&demo2Debug, "debug", false, "Enable debug mode")
	demo2Cmd.AddCommand(demo2UpCmd)
}

func openDemo2DebugLog() (*os.File, string, error) {
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

func demo2SafeUsername() string {
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

func validateDemo2ClusterName(name string) error {
	if !regexp.MustCompile(`^[a-zA-Z][-a-zA-Z0-9]*[a-zA-Z0-9]$`).MatchString(name) {
		return fmt.Errorf("cluster name must start with a letter, end with a letter or digit, and contain only letters, digits, or hyphens: got %q", name)
	}
	return nil
}

type demo2QoveryAPI struct {
	client *qovery.APIClient
}

func (a *demo2QoveryAPI) EnsureOnPremiseCredentials(ctx context.Context, organizationID string) (demo2Credential, error) {
	list, _, err := a.client.CloudProviderCredentialsAPI.ListOnPremiseCredentials(ctx, organizationID).Execute()
	if err != nil {
		return demo2Credential{}, err
	}
	if results := list.GetResults(); len(results) > 0 {
		return demo2CredentialFromQovery(results[0])
	}
	created, _, err := a.client.CloudProviderCredentialsAPI.CreateOnPremiseCredentials(ctx, organizationID).
		OnPremiseCredentialsRequest(qovery.OnPremiseCredentialsRequest{Name: "on-premise"}).
		Execute()
	if err != nil {
		return demo2Credential{}, err
	}
	return demo2CredentialFromQovery(*created)
}

func demo2CredentialFromQovery(value qovery.ClusterCredentials) (demo2Credential, error) {
	generic, ok := value.GetActualInstance().(*qovery.GenericClusterCredentials)
	if !ok || generic == nil || generic.Id == "" {
		return demo2Credential{}, errors.New("qovery returned invalid On-Premise credentials")
	}
	return demo2Credential{ID: generic.Id, Name: generic.Name}, nil
}

func (a *demo2QoveryAPI) FindCluster(ctx context.Context, organizationID string, name string) (string, bool, error) {
	clusters, _, err := a.client.ClustersAPI.ListOrganizationCluster(ctx, organizationID).Execute()
	if err != nil {
		return "", false, err
	}
	cluster := findCluster(clusters.GetResults(), name)
	if cluster == nil {
		return "", false, nil
	}
	return cluster.Id, true, nil
}

func newDemo2ClusterRequest(name string, credential demo2Credential) qovery.ClusterRequest {
	production := false
	kubernetes := qovery.KUBERNETESENUM_SELF_MANAGED
	provider := qovery.CLOUDPROVIDERENUM_ON_PREMISE
	region := "unknown"
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
		Features:             []qovery.ClusterRequestFeaturesInner{},
		AdditionalProperties: map[string]interface{}{"is_demo": true},
	}
}

func (a *demo2QoveryAPI) CreateCluster(ctx context.Context, organizationID string, name string, credential demo2Credential) (string, error) {
	cluster, _, err := a.client.ClustersAPI.CreateCluster(ctx, organizationID).
		ClusterRequest(newDemo2ClusterRequest(name, credential)).
		Execute()
	if err != nil {
		return "", err
	}
	return cluster.Id, nil
}

func (a *demo2QoveryAPI) ConfigureOperator(ctx context.Context, organizationID string, clusterID string, cpuArchitecture string) error {
	demoBinding, err := a.demoPlatformBinding(ctx, organizationID)
	if err != nil {
		return err
	}
	existingBinding, response, err := a.client.PlatformConfigurationAPI.GetClusterPlatformBinding(ctx, organizationID, clusterID).Execute()
	if err != nil {
		if response == nil || response.StatusCode != http.StatusNotFound {
			return err
		}
	}
	binding := selectDemo2PlatformBinding(existingBinding, demoBinding)
	request, err := newDemo2OperatorBindingRequest(binding, cpuArchitecture)
	if err != nil {
		return err
	}
	_, _, err = a.client.PlatformConfigurationAPI.UpdateClusterPlatformBinding(ctx, organizationID, clusterID).
		ClusterPlatformBindingRequest(request).
		Execute()
	return err
}

func (a *demo2QoveryAPI) demoPlatformBinding(ctx context.Context, organizationID string) (*qovery.ClusterPlatformBindingResponse, error) {
	catalog, _, err := a.client.PlatformConfigurationAPI.ListPlatformTemplates(ctx, organizationID).
		ClusterMode(qovery.PLATFORMCLUSTERMODE_CUSTOMER_MANAGED).
		CloudProvider(qovery.PLATFORMCLOUDVENDOR_UNKNOWN).
		Execute()
	if err != nil {
		return nil, err
	}
	return newDemo2PlatformBinding(catalog)
}

func newDemo2PlatformBinding(catalog *qovery.PlatformTemplateCatalogResponse) (*qovery.ClusterPlatformBindingResponse, error) {
	if catalog == nil {
		return nil, fmt.Errorf("qovery returned no %q platform template for the local demo", demo2PlatformTemplateKey)
	}
	var template *qovery.PlatformTemplateSummaryResponse
	for i := range catalog.Results {
		candidate := &catalog.Results[i]
		if candidate.Key == demo2PlatformTemplateKey && strings.TrimSpace(candidate.Version) != "" {
			template = candidate
			break
		}
	}
	if template == nil {
		return nil, fmt.Errorf("qovery returned no %q platform template for the local demo", demo2PlatformTemplateKey)
	}
	layerSelections := make(map[string]bool)
	for _, layer := range template.Layers {
		if !layer.Mandatory {
			layerSelections[layer.Key] = layer.EnabledByDefault
		}
	}
	return &qovery.ClusterPlatformBindingResponse{
		TemplateKey:            template.Key,
		TemplateVersion:        template.Version,
		LayerSelections:        layerSelections,
		ManagedConfig:          map[string]map[string]interface{}{},
		CustomerProvidedInputs: map[string]map[string]string{},
	}, nil
}

func selectDemo2PlatformBinding(
	existingBinding *qovery.ClusterPlatformBindingResponse,
	demoBinding *qovery.ClusterPlatformBindingResponse,
) *qovery.ClusterPlatformBindingResponse {
	if existingBinding != nil && demoBinding != nil &&
		existingBinding.TemplateKey == demoBinding.TemplateKey &&
		existingBinding.TemplateVersion == demoBinding.TemplateVersion {
		return existingBinding
	}
	return demoBinding
}

func newDemo2OperatorBindingRequest(binding *qovery.ClusterPlatformBindingResponse, cpuArchitecture string) (qovery.ClusterPlatformBindingRequest, error) {
	if binding == nil || strings.TrimSpace(binding.TemplateKey) == "" || strings.TrimSpace(binding.TemplateVersion) == "" {
		return qovery.ClusterPlatformBindingRequest{}, errors.New("qovery returned an invalid platform binding")
	}
	architecture := strings.ToUpper(strings.TrimSpace(cpuArchitecture))
	if architecture != "AMD64" && architecture != "ARM64" {
		return qovery.ClusterPlatformBindingRequest{}, fmt.Errorf("unsupported local CPU architecture %q", cpuArchitecture)
	}

	managedConfig := make(map[string]map[string]interface{}, len(binding.ManagedConfig)+1)
	for component, values := range binding.ManagedConfig {
		managedConfig[component] = make(map[string]interface{}, len(values))
		for key, value := range values {
			managedConfig[component][key] = value
		}
	}
	operatorConfig := managedConfig["qovery-operator"]
	if operatorConfig == nil {
		operatorConfig = map[string]interface{}{}
	}
	operatorConfig["cpuArchitectures"] = architecture
	managedConfig["qovery-operator"] = operatorConfig

	request := qovery.NewClusterPlatformBindingRequest(binding.TemplateKey, binding.TemplateVersion)
	request.SetLayerSelections(binding.LayerSelections)
	request.SetManagedConfig(managedConfig)
	request.SetCustomerProvidedInputs(binding.CustomerProvidedInputs)
	return *request, nil
}

func (a *demo2QoveryAPI) GetOperatorBootstrap(ctx context.Context, organizationID string, clusterID string) (demo2Bootstrap, error) {
	bootstrap, _, err := a.client.ClusterOperatorAPI.GetClusterOperatorBootstrap(ctx, organizationID, clusterID).Execute()
	if err != nil {
		return demo2Bootstrap{}, err
	}
	return demo2Bootstrap{
		ReleaseName:    bootstrap.ReleaseName,
		ChartReference: bootstrap.ChartReference,
		ChartVersion:   bootstrap.ChartVersion,
		Namespace:      bootstrap.Namespace,
		ValuesYAML:     bootstrap.ValuesYaml,
	}, nil
}

func (a *demo2QoveryAPI) AttachOperator(ctx context.Context, organizationID string, clusterID string) error {
	_, err := a.client.ClusterOperatorAPI.AttachClusterOperator(ctx, organizationID, clusterID).Execute()
	return err
}

func (a *demo2QoveryAPI) GetOperatorStatus(ctx context.Context, organizationID string, clusterID string) (demo2OperatorStatus, error) {
	status, _, err := a.client.ClusterOperatorAPI.GetClusterOperatorStatus(ctx, organizationID, clusterID).Execute()
	if err != nil {
		return demo2OperatorStatus{}, err
	}
	return demo2OperatorStatus{
		Connected:     status.OperatorConnected,
		LastHeartbeat: status.LastHeartbeat.Get(),
	}, nil
}

func (a *demo2QoveryAPI) DeployCluster(ctx context.Context, organizationID string, clusterID string) (string, error) {
	status, _, err := a.client.ClustersAPI.DeployCluster(ctx, organizationID, clusterID).Execute()
	if err != nil {
		return "", err
	}
	return string(status.Status), nil
}

func (a *demo2QoveryAPI) GetClusterStatus(ctx context.Context, organizationID string, clusterID string) (string, error) {
	status, _, err := a.client.ClustersAPI.GetClusterStatus(ctx, organizationID, clusterID).Execute()
	if err != nil {
		return "", err
	}
	return string(status.Status), nil
}

type demo2CommandRunner interface {
	LookPath(string) error
	Run(context.Context, string, ...string) ([]byte, error)
	RunQuiet(context.Context, string, ...string) ([]byte, error)
}

type demo2ExecRunner struct {
	out   io.Writer
	log   io.Writer
	debug bool
}

func (r *demo2ExecRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (r *demo2ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.run(ctx, true, name, args...)
}

func (r *demo2ExecRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.run(ctx, false, name, args...)
}

func (r *demo2ExecRunner) run(ctx context.Context, visible bool, name string, args ...string) ([]byte, error) {
	commandLine := strings.Join(append([]string{name}, args...), " ")
	if r.log != nil {
		_, _ = fmt.Fprintf(r.log, "$ %s\n", commandLine)
	}
	if (visible || r.debug) && r.out != nil {
		_, _ = fmt.Fprintf(r.out, "$ %s\n", commandLine)
	}

	var output bytes.Buffer
	writers := []io.Writer{&output}
	if r.log != nil {
		writers = append(writers, r.log)
	}
	if (visible || r.debug) && r.out != nil {
		writers = append(writers, r.out)
	}
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = io.MultiWriter(writers...)
	command.Stderr = command.Stdout
	err := command.Run()
	if err != nil && !visible && !r.debug && r.out != nil {
		_, _ = fmt.Fprintf(r.out, "$ %s\n", commandLine)
		_, _ = r.out.Write(output.Bytes())
	}
	return output.Bytes(), err
}

type demo2LocalCommands struct {
	runner  demo2CommandRunner
	goos    string
	tempDir string
}

func (l *demo2LocalCommands) CheckDependencies(ctx context.Context) error {
	for _, dependency := range []string{"docker", "k3d", "helm", "kubectl"} {
		if err := l.runner.LookPath(dependency); err != nil {
			return fmt.Errorf("required command %q is not installed", dependency)
		}
	}
	if _, err := l.runner.RunQuiet(ctx, "docker", "info"); err != nil {
		return errors.New("docker is not running")
	}
	return nil
}

func (l *demo2LocalCommands) EnsureK3dCluster(ctx context.Context, name string) error {
	output, err := l.runner.RunQuiet(ctx, "k3d", "cluster", "list", "--output", "json")
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
			return l.ensureDemo2Registry(ctx, name)
		}
	}
	args := []string{
		"cluster", "create", name,
		"--image", demo2K3sImage,
		"--subnet", demo2Subnet,
		"--k3s-arg", "--node-ip=" + demo2NodeIP + "@server:0",
		"--k3s-arg", "--disable=traefik@server:*",
		"--registry-create", demo2Registry,
		"--port", "80:80@loadbalancer",
		"--port", "443:443@loadbalancer",
	}
	if _, err := l.runner.Run(ctx, "k3d", args...); err != nil {
		return commandFailed("k3d cluster create", err)
	}
	return l.ensureDemo2Registry(ctx, name)
}

func (l *demo2LocalCommands) ensureDemo2Registry(ctx context.Context, clusterName string) error {
	containerName, found, err := l.findDemo2Registry(ctx)
	if err != nil {
		return err
	}
	if found {
		return l.ensureDemo2RegistryAlias(ctx, clusterName, containerName)
	}
	if _, err := l.runner.Run(
		ctx,
		"k3d", "registry", "create", demo2Registry,
		"--default-network", "k3d-"+clusterName,
		"--no-help",
	); err != nil {
		return commandFailed("k3d registry create", err)
	}
	containerName, found, err = l.findDemo2Registry(ctx)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("k3d did not expose the created demo registry")
	}
	return l.ensureDemo2RegistryAlias(ctx, clusterName, containerName)
}

func (l *demo2LocalCommands) findDemo2Registry(ctx context.Context) (string, bool, error) {
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
		if registry.Name == demo2Registry || registry.Name == "k3d-"+demo2Registry {
			return registry.Name, true, nil
		}
	}
	return "", false, nil
}

func (l *demo2LocalCommands) ensureDemo2RegistryAlias(ctx context.Context, clusterName string, containerName string) error {
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
	if connected && (containsString(network.Aliases, demo2Registry) || containsString(network.DNSNames, demo2Registry)) {
		return nil
	}
	if connected {
		if _, err := l.runner.Run(ctx, "docker", "network", "disconnect", networkName, containerName); err != nil {
			return commandFailed("docker network disconnect demo registry", err)
		}
	}
	if _, err := l.runner.Run(
		ctx,
		"docker", "network", "connect", "--alias", demo2Registry, networkName, containerName,
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

func (l *demo2LocalCommands) EnsureLoopback(ctx context.Context) error {
	switch l.goos {
	case "darwin":
		output, err := l.runner.RunQuiet(ctx, "ifconfig", "lo0")
		if err != nil {
			return commandFailed("macOS loopback inspection", err)
		}
		if strings.Contains(string(output), demo2NodeIP) {
			return nil
		}
		if _, err := l.runner.Run(ctx, "sudo", "ifconfig", "lo0", "alias", demo2NodeIP+"/32", "up"); err != nil {
			return commandFailed("macOS loopback configuration", err)
		}
	case "linux":
		version, err := os.ReadFile("/proc/version")
		if err == nil && strings.Contains(strings.ToLower(string(version)), "microsoft") {
			output, err := l.runner.Run(ctx, "sudo", "ip", "addr", "add", demo2NodeIP+"/32", "dev", "lo")
			if err != nil && !strings.Contains(strings.ToLower(string(output)), "exists") {
				return commandFailed("WSL loopback configuration", err)
			}
			powershell := "powershell.exe"
			if err := l.runner.LookPath(powershell); err != nil {
				powershell = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
			}
			command := fmt.Sprintf("Start-Process netsh -Verb RunAs -ArgumentList \"interface ipv4 add address name='Loopback Pseudo-Interface 1' address=%s mask=255.255.255.255 skipassource=true\"", demo2NodeIP)
			output, err = l.runner.Run(ctx, powershell, "-NoProfile", "-Command", command)
			if err != nil && !strings.Contains(strings.ToLower(string(output)), "exists") {
				return commandFailed("Windows loopback configuration", err)
			}
		}
	}
	return nil
}

func (l *demo2LocalCommands) LegacyQoveryReleaseExists(ctx context.Context) (bool, error) {
	output, err := l.runner.RunQuiet(ctx, "helm", "list", "--namespace", "qovery", "--all", "--output", "json", "--filter", "^qovery$")
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

func buildDemo2HelmArgs(bootstrap demo2Bootstrap, valuesPath string) ([]string, error) {
	if strings.TrimSpace(bootstrap.ReleaseName) == "" || strings.TrimSpace(bootstrap.ChartReference) == "" || strings.TrimSpace(bootstrap.ChartVersion) == "" || strings.TrimSpace(bootstrap.Namespace) == "" {
		return nil, errors.New("operator bootstrap is missing structured Helm fields")
	}
	if path.Base(strings.TrimSuffix(bootstrap.ChartReference, "/")) == "qovery" {
		return nil, errors.New("refusing to install the legacy Qovery umbrella chart")
	}
	return []string{
		"upgrade", "--install",
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

func (l *demo2LocalCommands) InstallOperator(ctx context.Context, bootstrap demo2Bootstrap) error {
	file, err := os.CreateTemp(l.tempDir, "qovery-demo2-operator-values-*.yaml")
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
	args, err := buildDemo2HelmArgs(bootstrap, path)
	if err != nil {
		return err
	}
	if _, err := l.runner.Run(ctx, "helm", args...); err != nil {
		return commandFailed("helm upgrade --install", err)
	}
	return nil
}

func (l *demo2LocalCommands) ValidateWorkloads(ctx context.Context, namespace string) error {
	output, err := l.runner.RunQuiet(ctx, "kubectl", "--namespace", namespace, "get", "deployments", "--output", "json")
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
			return errors.New("unexpected permanent Deployment qovery-engine exists in namespace qovery")
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

type demo2SystemClock struct{}

func (demo2SystemClock) Now() time.Time { return time.Now() }

func (demo2SystemClock) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
