package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	demoDestroyClusterName        string
	demoDestroyDeleteQoveryConfig bool
)

var demoDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the local demo cluster",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		context.AfterFunc(ctx, stop)
		err := runDemoDestroy(ctx, cmd)
		stop()
		if err != nil {
			utils.CaptureWithEvent(cmd, utils.EndOfExecutionErrorEventName)
			utils.PrintlnError(err)
			os.Exit(1)
		}
		utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
	},
}

func runDemoDestroy(ctx context.Context, cmd *cobra.Command) error {
	if runtime.GOOS == "windows" {
		return errors.New("qovery demo is not supported directly on Windows; use WSL")
	}
	if err := validateDemoClusterName(demoDestroyClusterName); err != nil {
		return err
	}

	var api demoDestroyAPI
	organizationID := ""
	if demoDestroyDeleteQoveryConfig {
		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			return fmt.Errorf("authentication failed; run `qovery auth` first: %w", err)
		}
		organization, _, err := utils.CurrentOrganization(true)
		if err != nil {
			return fmt.Errorf("cannot resolve the current organization: %w", err)
		}
		organizationID = string(organization)
		api = &demoQoveryAPI{client: utils.GetQoveryClient(tokenType, token)}
	}

	orchestrator := demoDestroyOrchestrator{
		api:   api,
		local: &demoLocalCommands{runner: &demoExecRunner{out: cmd.OutOrStdout()}, goos: runtime.GOOS},
		out:   cmd.OutOrStdout(),
	}
	return orchestrator.Destroy(ctx, demoDestroyConfig{
		OrganizationID:     organizationID,
		ClusterName:        demoDestroyClusterName,
		DeleteQoveryConfig: demoDestroyDeleteQoveryConfig,
	})
}

func init() {
	demoDestroyClusterName = "local-demo-" + demoSafeUsername()
	demoDestroyCmd.Flags().StringVarP(&demoDestroyClusterName, "cluster-name", "c", demoDestroyClusterName, "The name of the local demo cluster")
	demoDestroyCmd.Flags().BoolVarP(
		&demoDestroyDeleteQoveryConfig,
		"delete-qovery-config",
		"d",
		false,
		"Also delete the Qovery cluster, its environments, and its Operator association",
	)
	demoCmd.AddCommand(demoDestroyCmd)
}

type demoDestroyAPI interface {
	FindCluster(context.Context, string, string) (string, bool, error)
	DeleteClusterConfig(context.Context, string, string) error
}

type demoDestroyLocal interface {
	CheckDestroyDependencies(context.Context) error
	DeleteK3dCluster(context.Context, string) error
	TeardownLoopback(context.Context) error
}

type demoDestroyConfig struct {
	OrganizationID     string
	ClusterName        string
	DeleteQoveryConfig bool
}

type demoDestroyOrchestrator struct {
	api   demoDestroyAPI
	local demoDestroyLocal
	out   io.Writer
}

func (o *demoDestroyOrchestrator) Destroy(ctx context.Context, cfg demoDestroyConfig) error {
	o.phase("Checking local dependencies")
	if err := o.local.CheckDestroyDependencies(ctx); err != nil {
		return fmt.Errorf("local dependency check failed: %w", err)
	}

	clusterID, found := "", false
	var lookupErr error
	if cfg.DeleteQoveryConfig {
		if o.api == nil {
			return errors.New("qovery API is required to delete the remote configuration")
		}
		clusterID, found, lookupErr = o.api.FindCluster(ctx, cfg.OrganizationID, cfg.ClusterName)
		if errors.Is(lookupErr, errNotADemoCluster) {
			return lookupErr
		}
	}

	o.phase("Deleting the local k3d cluster and registry")
	if err := o.local.DeleteK3dCluster(ctx, cfg.ClusterName); err != nil {
		return fmt.Errorf("cannot delete local k3d resources: %w", err)
	}
	if err := o.local.TeardownLoopback(ctx); err != nil {
		_, _ = fmt.Fprintf(o.out, "Warning: cannot remove the local loopback address %s: %v\n", demoNodeIP, err)
	}

	if !cfg.DeleteQoveryConfig {
		_, _ = fmt.Fprintf(o.out, "Local demo cluster %q was deleted. Its Qovery configuration and Operator association were preserved.\n", cfg.ClusterName)
		return nil
	}

	o.phase("Deleting the Qovery cluster configuration")
	if lookupErr != nil {
		return fmt.Errorf("local demo cluster %q was deleted, but its Qovery cluster cannot be looked up; run the command again to delete it: %w", cfg.ClusterName, lookupErr)
	}
	if !found {
		_, _ = fmt.Fprintf(o.out, "Local demo cluster %q was deleted. No Qovery cluster with this name exists in the current organization.\n", cfg.ClusterName)
		return nil
	}
	if err := o.api.DeleteClusterConfig(ctx, cfg.OrganizationID, clusterID); err != nil {
		return fmt.Errorf("cannot delete Qovery cluster configuration: %w", err)
	}
	_, _ = fmt.Fprintf(o.out, "Demo cluster %q and its Qovery configuration were deleted.\n", cfg.ClusterName)
	return nil
}

func (o *demoDestroyOrchestrator) phase(message string) {
	_, _ = fmt.Fprintf(o.out, "\n==> %s\n", message)
}

func (a *demoQoveryAPI) DeleteClusterConfig(ctx context.Context, organizationID string, clusterID string) error {
	_, err := callDemoAPIWithoutBody(a, func(client *qovery.APIClient) (*http.Response, error) {
		return client.ClustersAPI.DeleteCluster(ctx, organizationID, clusterID).
			DeleteMode(qovery.CLUSTERDELETEMODE_DELETE_QOVERY_CONFIG).
			Execute()
	})
	return err
}

func (l *demoLocalCommands) CheckDestroyDependencies(_ context.Context) error {
	for _, dependency := range []string{"docker", "k3d"} {
		if err := l.runner.LookPath(dependency); err != nil {
			return fmt.Errorf("required command %q is not installed", dependency)
		}
	}
	return nil
}

func (l *demoLocalCommands) DeleteK3dCluster(ctx context.Context, name string) error {
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
	registryShared, err := l.demoRegistryUsedOutsideCluster(ctx, name)
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		if cluster.Name == name {
			if _, err := l.runner.Run(ctx, "k3d", "cluster", "delete", name); err != nil {
				return commandFailed("k3d cluster delete", err)
			}
			break
		}
	}
	if registryShared {
		return nil
	}
	return l.deleteDemoRegistry(ctx)
}

func (l *demoLocalCommands) demoRegistryUsedOutsideCluster(ctx context.Context, clusterName string) (bool, error) {
	containerName, found, err := l.findDemoRegistry(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	output, err := l.runner.RunQuiet(ctx, "docker", "inspect", containerName)
	if err != nil {
		return false, commandFailed("docker inspect demo registry", err)
	}
	var containers []struct {
		NetworkSettings struct {
			Networks map[string]json.RawMessage `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(output, &containers); err != nil || len(containers) != 1 {
		return false, errors.New("docker returned invalid demo registry details")
	}
	targetNetwork := "k3d-" + clusterName
	for network := range containers[0].NetworkSettings.Networks {
		if strings.HasPrefix(network, "k3d-") && network != targetNetwork {
			return true, nil
		}
	}
	return false, nil
}

func (l *demoLocalCommands) deleteDemoRegistry(ctx context.Context) error {
	containerName, found, err := l.findDemoRegistry(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if _, err := l.runner.Run(ctx, "k3d", "registry", "delete", containerName); err != nil {
		return commandFailed("k3d registry delete", err)
	}
	return nil
}

func (l *demoLocalCommands) TeardownLoopback(ctx context.Context) error {
	if l.goos == "darwin" {
		output, err := l.runner.RunQuiet(ctx, "ifconfig", "lo0")
		if err != nil {
			return commandFailed("macOS loopback inspection", err)
		}
		if !loopbackHasAddress(output, demoNodeIP) {
			return nil
		}
		if _, err := l.runner.Run(ctx, "sudo", "ifconfig", "lo0", "-alias", demoNodeIP); err != nil {
			return commandFailed("macOS loopback removal", err)
		}
		return nil
	}
	if l.goos != "linux" {
		return nil
	}
	version, err := os.ReadFile("/proc/version")
	if err != nil || !strings.Contains(strings.ToLower(string(version)), "microsoft") {
		return nil
	}
	output, err := l.runner.RunQuiet(ctx, "ip", "addr", "show", "dev", "lo")
	if err != nil {
		return commandFailed("WSL loopback inspection", err)
	}
	if loopbackHasAddress(output, demoNodeIP) {
		if _, err := l.runner.Run(ctx, "sudo", "ip", "addr", "del", demoNodeIP+"/32", "dev", "lo"); err != nil {
			return commandFailed("WSL loopback removal", err)
		}
	}
	powershell := "powershell.exe"
	if err := l.runner.LookPath(powershell); err != nil {
		powershell = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	}
	command := fmt.Sprintf("Start-Process netsh -Verb RunAs -Wait -ArgumentList \"interface ipv4 delete address name='Loopback Pseudo-Interface 1' address=%s\"", demoNodeIP)
	if _, err := l.runner.Run(ctx, powershell, "-NoProfile", "-Command", command); err != nil {
		return commandFailed("Windows loopback removal", err)
	}
	return nil
}
