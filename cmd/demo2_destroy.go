package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	demo2DestroyClusterName        string
	demo2DestroyDeleteQoveryConfig bool
)

var demo2DestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the experimental local Operator demo",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.Capture(cmd)
		if runtime.GOOS == "windows" {
			return errors.New("qovery demo2 is not supported directly on Windows; use WSL")
		}
		if err := validateDemo2ClusterName(demo2DestroyClusterName); err != nil {
			return err
		}

		var api demo2DestroyAPI
		organizationID := ""
		if demo2DestroyDeleteQoveryConfig {
			tokenType, token, err := utils.GetAccessToken()
			if err != nil {
				return fmt.Errorf("authentication failed; run `qovery auth` first: %w", err)
			}
			organization, _, err := utils.CurrentOrganization(true)
			if err != nil {
				return fmt.Errorf("cannot resolve the current organization: %w", err)
			}
			organizationID = string(organization)
			api = &demo2QoveryAPI{client: utils.GetQoveryClient(tokenType, token)}
		}

		orchestrator := demo2DestroyOrchestrator{
			api:   api,
			local: &demo2LocalCommands{runner: &demo2ExecRunner{}, goos: runtime.GOOS},
			out:   cmd.OutOrStdout(),
		}
		err := orchestrator.Destroy(cmd.Context(), demo2DestroyConfig{
			OrganizationID:     organizationID,
			ClusterName:        demo2DestroyClusterName,
			DeleteQoveryConfig: demo2DestroyDeleteQoveryConfig,
		})
		if err == nil {
			utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
		}
		return err
	},
}

func init() {
	demo2DestroyClusterName = "local-demo2-" + demo2SafeUsername()
	demo2DestroyCmd.Flags().StringVarP(&demo2DestroyClusterName, "cluster-name", "c", demo2DestroyClusterName, "The name of the experimental local cluster")
	demo2DestroyCmd.Flags().BoolVarP(
		&demo2DestroyDeleteQoveryConfig,
		"delete-qovery-config",
		"d",
		false,
		"Also delete the Qovery cluster, its environments, and its Operator association",
	)
	demo2Cmd.AddCommand(demo2DestroyCmd)
}

type demo2DestroyAPI interface {
	FindCluster(context.Context, string, string) (string, bool, error)
	DeleteClusterConfig(context.Context, string, string) error
}

type demo2DestroyLocal interface {
	CheckDestroyDependencies(context.Context) error
	DeleteK3dCluster(context.Context, string) error
	TeardownLoopback(context.Context) error
}

type demo2DestroyConfig struct {
	OrganizationID     string
	ClusterName        string
	DeleteQoveryConfig bool
}

type demo2DestroyOrchestrator struct {
	api   demo2DestroyAPI
	local demo2DestroyLocal
	out   io.Writer
}

func (o *demo2DestroyOrchestrator) Destroy(ctx context.Context, cfg demo2DestroyConfig) error {
	o.phase("Checking local dependencies")
	if err := o.local.CheckDestroyDependencies(ctx); err != nil {
		return fmt.Errorf("local dependency check failed: %w", err)
	}

	o.phase("Deleting the local k3d cluster and registry")
	if err := o.local.DeleteK3dCluster(ctx, cfg.ClusterName); err != nil {
		return fmt.Errorf("cannot delete local k3d resources: %w", err)
	}
	if err := o.local.TeardownLoopback(ctx); err != nil {
		return fmt.Errorf("cannot remove local loopback configuration: %w", err)
	}

	if !cfg.DeleteQoveryConfig {
		_, _ = fmt.Fprintf(o.out, "Local Demo2 cluster %q was deleted. Its Qovery configuration and Operator association were preserved.\n", cfg.ClusterName)
		return nil
	}
	if o.api == nil {
		return errors.New("qovery API is required to delete the remote configuration")
	}

	o.phase("Deleting the Qovery cluster configuration")
	clusterID, found, err := o.api.FindCluster(ctx, cfg.OrganizationID, cfg.ClusterName)
	if err != nil {
		return fmt.Errorf("cannot look up Qovery cluster: %w", err)
	}
	if found {
		if err := o.api.DeleteClusterConfig(ctx, cfg.OrganizationID, clusterID); err != nil {
			return fmt.Errorf("cannot delete Qovery cluster configuration: %w", err)
		}
	}
	_, _ = fmt.Fprintf(o.out, "Demo2 cluster %q and its Qovery configuration were deleted.\n", cfg.ClusterName)
	return nil
}

func (o *demo2DestroyOrchestrator) phase(message string) {
	_, _ = fmt.Fprintf(o.out, "\n==> %s\n", message)
}

func (a *demo2QoveryAPI) DeleteClusterConfig(ctx context.Context, organizationID string, clusterID string) error {
	_, err := a.client.ClustersAPI.DeleteCluster(ctx, organizationID, clusterID).
		DeleteMode(qovery.CLUSTERDELETEMODE_DELETE_QOVERY_CONFIG).
		Execute()
	return err
}

func (l *demo2LocalCommands) CheckDestroyDependencies(_ context.Context) error {
	for _, dependency := range []string{"docker", "k3d"} {
		if err := l.runner.LookPath(dependency); err != nil {
			return fmt.Errorf("required command %q is not installed", dependency)
		}
	}
	return nil
}

func (l *demo2LocalCommands) DeleteK3dCluster(ctx context.Context, name string) error {
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
	registryShared, err := l.demo2RegistryUsedOutsideCluster(ctx, name)
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
	return l.deleteDemo2Registry(ctx)
}

func (l *demo2LocalCommands) demo2RegistryUsedOutsideCluster(ctx context.Context, clusterName string) (bool, error) {
	containerName, found, err := l.findDemo2Registry(ctx)
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

func (l *demo2LocalCommands) deleteDemo2Registry(ctx context.Context) error {
	containerName, found, err := l.findDemo2Registry(ctx)
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

func (l *demo2LocalCommands) TeardownLoopback(ctx context.Context) error {
	if l.goos == "darwin" {
		output, err := l.runner.RunQuiet(ctx, "ifconfig", "lo0")
		if err != nil {
			return commandFailed("macOS loopback inspection", err)
		}
		if !strings.Contains(string(output), demo2NodeIP) {
			return nil
		}
		if _, err := l.runner.Run(ctx, "sudo", "ifconfig", "lo0", "-alias", demo2NodeIP); err != nil {
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
	output, err := l.runner.RunQuiet(ctx, "sudo", "ip", "addr", "show", "dev", "lo")
	if err != nil {
		return commandFailed("WSL loopback inspection", err)
	}
	if strings.Contains(string(output), demo2NodeIP) {
		if _, err := l.runner.Run(ctx, "sudo", "ip", "addr", "del", demo2NodeIP+"/32", "dev", "lo"); err != nil {
			return commandFailed("WSL loopback removal", err)
		}
	}
	powershell := "powershell.exe"
	if err := l.runner.LookPath(powershell); err != nil {
		powershell = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	}
	command := fmt.Sprintf("Start-Process netsh -Verb RunAs -ArgumentList \"interface ipv4 delete address name='Loopback Pseudo-Interface 1' address=%s\"", demo2NodeIP)
	if _, err := l.runner.Run(ctx, powershell, "-NoProfile", "-Command", command); err != nil {
		return commandFailed("Windows loopback removal", err)
	}
	return nil
}
