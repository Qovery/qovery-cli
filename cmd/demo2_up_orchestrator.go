package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	demo2OperatorHeartbeatFreshness = 2 * time.Minute
	demo2DefaultOperatorTimeout     = 10 * time.Minute
	demo2DefaultDeploymentTimeout   = 90 * time.Minute
	demo2DefaultPollInterval        = 5 * time.Second
)

type demo2Credential struct {
	ID   string
	Name string
}

type demo2Bootstrap struct {
	ReleaseName    string
	ChartReference string
	ChartVersion   string
	Namespace      string
	ValuesYAML     string
}

type demo2OperatorStatus struct {
	Connected     bool
	LastHeartbeat *time.Time
}

type demo2API interface {
	EnsureOnPremiseCredentials(context.Context, string) (demo2Credential, error)
	FindCluster(context.Context, string, string) (string, bool, error)
	CreateCluster(context.Context, string, string, demo2Credential) (string, error)
	ConfigureOperator(context.Context, string, string, string) error
	GetOperatorBootstrap(context.Context, string, string) (demo2Bootstrap, error)
	AttachOperator(context.Context, string, string) error
	GetOperatorStatus(context.Context, string, string) (demo2OperatorStatus, error)
	DeployCluster(context.Context, string, string) (string, error)
	GetClusterStatus(context.Context, string, string) (string, error)
}

type demo2Local interface {
	CheckDependencies(context.Context) error
	EnsureK3dCluster(context.Context, string) error
	EnsureLoopback(context.Context) error
	LegacyQoveryReleaseExists(context.Context) (bool, error)
	InstallOperator(context.Context, demo2Bootstrap) error
	ValidateWorkloads(context.Context, string) error
}

type demo2Clock interface {
	Now() time.Time
	Sleep(context.Context, time.Duration) error
}

type demo2Config struct {
	OrganizationID    string
	ClusterName       string
	CPUArchitecture   string
	OperatorTimeout   time.Duration
	DeploymentTimeout time.Duration
	PollInterval      time.Duration
}

type demo2Orchestrator struct {
	api   demo2API
	local demo2Local
	clock demo2Clock
	out   io.Writer
}

func (o *demo2Orchestrator) Up(ctx context.Context, cfg demo2Config) error {
	if cfg.OperatorTimeout <= 0 {
		cfg.OperatorTimeout = demo2DefaultOperatorTimeout
	}
	if cfg.DeploymentTimeout <= 0 {
		cfg.DeploymentTimeout = demo2DefaultDeploymentTimeout
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = demo2DefaultPollInterval
	}

	o.phase("Checking local dependencies")
	if err := o.local.CheckDependencies(ctx); err != nil {
		return fmt.Errorf("local dependency check failed: %w", err)
	}

	o.phase("Resolving Qovery On-Premise credentials and cluster")
	credential, err := o.api.EnsureOnPremiseCredentials(ctx, cfg.OrganizationID)
	if err != nil {
		return fmt.Errorf("cannot resolve On-Premise credentials: %w", err)
	}
	clusterID, found, err := o.api.FindCluster(ctx, cfg.OrganizationID, cfg.ClusterName)
	if err != nil {
		return fmt.Errorf("cannot look up Qovery cluster: %w", err)
	}
	if !found {
		clusterID, err = o.api.CreateCluster(ctx, cfg.OrganizationID, cfg.ClusterName, credential)
		if err != nil {
			return fmt.Errorf("cannot create Qovery cluster: %w", err)
		}
	}

	o.phase("Creating or starting the local k3d cluster")
	if err := o.local.EnsureK3dCluster(ctx, cfg.ClusterName); err != nil {
		return fmt.Errorf("cannot prepare local k3d cluster: %w", err)
	}
	if err := o.local.EnsureLoopback(ctx); err != nil {
		return fmt.Errorf("cannot configure local loopback: %w", err)
	}

	o.phase("Checking for an unsupported legacy Qovery release")
	legacy, err := o.local.LegacyQoveryReleaseExists(ctx)
	if err != nil {
		return fmt.Errorf("cannot inspect Helm releases: %w", err)
	}
	if legacy {
		return errors.New("legacy Helm release \"qovery\" exists in namespace \"qovery\"; adopting an old demo is not supported: destroy and recreate the local cluster before running `qovery demo2 up`")
	}

	o.phase("Bootstrapping and attaching the Qovery Operator")
	if err := o.api.ConfigureOperator(ctx, cfg.OrganizationID, clusterID, cfg.CPUArchitecture); err != nil {
		return fmt.Errorf("cannot configure the Qovery Operator for the local demo: %w", err)
	}
	bootstrap, err := o.api.GetOperatorBootstrap(ctx, cfg.OrganizationID, clusterID)
	if err != nil {
		return errors.New("cannot retrieve the Qovery Operator bootstrap")
	}
	if err := o.api.AttachOperator(ctx, cfg.OrganizationID, clusterID); err != nil {
		return fmt.Errorf("cannot attach the cluster to the Qovery Operator path: %w", err)
	}
	if err := o.local.InstallOperator(ctx, bootstrap); err != nil {
		return errors.New("operator Helm installation failed; sensitive bootstrap values were redacted")
	}

	o.phase("Waiting for a fresh Qovery Operator heartbeat")
	if err := o.waitForOperator(ctx, cfg, clusterID); err != nil {
		return err
	}

	o.phase("Deploying the current self-managed platform catalog")
	initialStatus, err := o.api.DeployCluster(ctx, cfg.OrganizationID, clusterID)
	if err != nil {
		return fmt.Errorf("cannot trigger cluster deployment: %w", err)
	}
	status, err := o.waitForDeployment(ctx, cfg, clusterID, initialStatus)
	if err != nil {
		return err
	}

	o.phase("Verifying Operator and Engine workloads")
	if err := o.local.ValidateWorkloads(ctx, bootstrap.Namespace); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(
		o.out,
		"\nQovery demo cluster is now installed !!!!\nThe kubeconfig is correctly set, so you can connect to it directly with kubectl or k9s from your local machine.\nTo delete/stop/start your cluster, use k3d cluster xxxx.\n\nGo to https://console.qovery.com to create your first environment on this cluster %q.\nCluster deployment finished with status %s.\n",
		cfg.ClusterName,
		status,
	)
	return nil
}

func (o *demo2Orchestrator) waitForOperator(ctx context.Context, cfg demo2Config, clusterID string) error {
	deadline := o.clock.Now().Add(cfg.OperatorTimeout)
	for {
		status, err := o.api.GetOperatorStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return fmt.Errorf("cannot read Qovery Operator status: %w", err)
		}
		if operatorStatusReady(status, o.clock.Now()) {
			return nil
		}
		if !o.clock.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for a connected Qovery Operator with a fresh heartbeat", cfg.OperatorTimeout)
		}
		if err := o.clock.Sleep(ctx, cfg.PollInterval); err != nil {
			return err
		}
	}
}

func operatorStatusReady(status demo2OperatorStatus, now time.Time) bool {
	if !status.Connected || status.LastHeartbeat == nil {
		return false
	}
	age := now.Sub(*status.LastHeartbeat)
	return age >= -30*time.Second && age <= demo2OperatorHeartbeatFreshness
}

func (o *demo2Orchestrator) waitForDeployment(ctx context.Context, cfg demo2Config, clusterID string, initialStatus string) (string, error) {
	deadline := o.clock.Now().Add(cfg.DeploymentTimeout)
	status := initialStatus
	for {
		if status == "DEPLOYED" || status == "RESTARTED" {
			return status, nil
		}
		if isDemo2DeploymentError(status) {
			return "", fmt.Errorf("cluster deployment finished with terminal status %s", status)
		}
		if !o.clock.Now().Before(deadline) {
			return "", fmt.Errorf("timed out after %s waiting for cluster deployment; last status was %s", cfg.DeploymentTimeout, status)
		}
		if err := o.clock.Sleep(ctx, cfg.PollInterval); err != nil {
			return "", err
		}
		var err error
		status, err = o.api.GetClusterStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return "", fmt.Errorf("cannot read cluster deployment status: %w", err)
		}
	}
}

func isDemo2DeploymentError(status string) bool {
	return strings.HasSuffix(status, "_ERROR") || status == "INVALID_CREDENTIALS" || status == "CANCELED" || status == "STOPPED" || status == "DELETED"
}

func (o *demo2Orchestrator) phase(message string) {
	const separator = `""""""""""""""""""""""""""""""""""""""""""""`
	_, _ = fmt.Fprintf(o.out, "\n%s\n%s\n%s\n", separator, message, separator)
}
