package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	qovery "github.com/qovery/qovery-client-go"
)

const (
	demoOperatorHeartbeatFreshness = 2 * time.Minute
	demoDefaultOperatorTimeout     = 10 * time.Minute
	demoDefaultDeploymentTimeout   = 90 * time.Minute
	demoDefaultPollInterval        = 5 * time.Second

	demoPhaseLocalDependencies         = "local_dependencies"
	demoPhaseCredentialsAndCluster     = "credentials_and_cluster"
	demoPhaseLocalCluster              = "local_cluster"
	demoPhaseLegacyReleaseCheck        = "legacy_release_check"
	demoPhaseOperatorBootstrap         = "operator_bootstrap"
	demoPhaseOperatorHeartbeat         = "operator_heartbeat"
	demoPhasePlatformCatalogDeployment = "platform_catalog_deployment"
	demoPhaseWorkloadVerification      = "workload_verification"
	demoPhaseResultSucceeded           = "succeeded"
	demoPhaseResultFailed              = "failed"
	demoPhaseResultCanceled            = "canceled"
)

type demoCredential struct {
	ID   string
	Name string
}

type demoBootstrap struct {
	ReleaseName    string
	ChartReference string
	ChartVersion   string
	Namespace      string
	ValuesYAML     string
}

type demoOperatorStatus struct {
	Connected     bool
	LastHeartbeat *time.Time
}

type demoAPI interface {
	EnsureOnPremiseCredentials(context.Context, string) (demoCredential, error)
	FindCluster(context.Context, string, string) (string, bool, error)
	CreateCluster(context.Context, string, string, demoCredential) (string, error)
	ConfigureOperator(context.Context, string, string, string) error
	GetOperatorBootstrap(context.Context, string, string) (demoBootstrap, error)
	AttachOperator(context.Context, string, string) error
	GetOperatorStatus(context.Context, string, string) (demoOperatorStatus, error)
	DeployCluster(context.Context, string, string) (string, error)
	GetClusterStatus(context.Context, string, string) (string, error)
}

type demoLocal interface {
	CheckDependencies(context.Context) error
	EnsureK3dCluster(context.Context, string) error
	EnsureLoopback(context.Context) error
	LegacyQoveryReleaseExists(context.Context) (bool, error)
	InstallOperator(context.Context, demoBootstrap) error
	ValidateWorkloads(context.Context, string) error
}

type demoClock interface {
	Now() time.Time
	Sleep(context.Context, time.Duration) error
}

type demoPhaseExecution struct {
	Phase            string
	Result           string
	Duration         time.Duration
	ErrorCode        string
	SafeErrorMessage string
}

type demoProgress interface {
	ClusterResolved(string)
	PhaseFinished(demoPhaseExecution)
}

type demoConfig struct {
	OrganizationID    string
	ClusterName       string
	CPUArchitecture   string
	OperatorTimeout   time.Duration
	DeploymentTimeout time.Duration
	PollInterval      time.Duration
}

type demoOrchestrator struct {
	api      demoAPI
	local    demoLocal
	clock    demoClock
	out      io.Writer
	progress demoProgress
}

func (o *demoOrchestrator) Up(ctx context.Context, cfg demoConfig) error {
	if cfg.OperatorTimeout <= 0 {
		cfg.OperatorTimeout = demoDefaultOperatorTimeout
	}
	if cfg.DeploymentTimeout <= 0 {
		cfg.DeploymentTimeout = demoDefaultDeploymentTimeout
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = demoDefaultPollInterval
	}

	if err := o.runPhase(ctx, demoPhaseLocalDependencies, "Checking local dependencies", func() error {
		if err := o.local.CheckDependencies(ctx); err != nil {
			return fmt.Errorf("local dependency check failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	var clusterID string
	if err := o.runPhase(ctx, demoPhaseCredentialsAndCluster, "Resolving Qovery On-Premise credentials and cluster", func() error {
		credential, err := o.api.EnsureOnPremiseCredentials(ctx, cfg.OrganizationID)
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot resolve On-Premise credentials: %w", err))
		}
		var found bool
		clusterID, found, err = o.api.FindCluster(ctx, cfg.OrganizationID, cfg.ClusterName)
		if errors.Is(err, errNotADemoCluster) {
			return newDemoFailure(
				demoErrorClusterNameNotADemo,
				"A Qovery cluster with this name exists and is not a demo cluster.",
				fmt.Errorf("%w; choose another name with --cluster-name", err),
			)
		}
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot look up Qovery cluster: %w", err))
		}
		if !found {
			clusterID, err = o.api.CreateCluster(ctx, cfg.OrganizationID, cfg.ClusterName, credential)
			if err != nil {
				return demoQoveryAPIFailure(fmt.Errorf("cannot create Qovery cluster: %w", err))
			}
		}
		o.clusterResolved(clusterID)
		return nil
	}); err != nil {
		return err
	}

	if err := o.runPhase(ctx, demoPhaseLocalCluster, "Creating or starting the local k3d cluster", func() error {
		if err := o.local.EnsureK3dCluster(ctx, cfg.ClusterName); err != nil {
			return newDemoFailure(
				demoErrorK3dCluster,
				"The local k3d cluster could not be created or started.",
				fmt.Errorf("cannot prepare local k3d cluster: %w", err),
			)
		}
		if err := o.local.EnsureLoopback(ctx); err != nil {
			return newDemoFailure(
				demoErrorLoopback,
				"The local loopback address could not be configured.",
				fmt.Errorf("cannot configure local loopback: %w", err),
			)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := o.runPhase(ctx, demoPhaseLegacyReleaseCheck, "Checking for an unsupported legacy Qovery release", func() error {
		legacy, err := o.local.LegacyQoveryReleaseExists(ctx)
		if err != nil {
			return newDemoFailure(
				demoErrorHelmList,
				"Listing the local Helm releases failed.",
				fmt.Errorf("cannot inspect Helm releases: %w", err),
			)
		}
		if legacy {
			return newDemoFailure(
				demoErrorLegacyRelease,
				"A legacy Qovery demo release exists on the local cluster.",
				errors.New("legacy Helm release \"qovery\" exists in namespace \"qovery\"; adopting an old demo is not supported: destroy and recreate the local cluster before running `qovery demo up`"),
			)
		}
		return nil
	}); err != nil {
		return err
	}

	var bootstrap demoBootstrap
	if err := o.runPhase(ctx, demoPhaseOperatorBootstrap, "Bootstrapping and attaching the Qovery Operator", func() error {
		if err := o.api.ConfigureOperator(ctx, cfg.OrganizationID, clusterID, cfg.CPUArchitecture); err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot configure the Qovery Operator for the local demo: %w", err))
		}
		var err error
		bootstrap, err = o.api.GetOperatorBootstrap(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return demoQoveryAPIFailure(errors.New("cannot retrieve the Qovery Operator bootstrap"))
		}
		if err := o.api.AttachOperator(ctx, cfg.OrganizationID, clusterID); err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot attach the cluster to the Qovery Operator path: %w", err))
		}
		if err := o.local.InstallOperator(ctx, bootstrap); err != nil {
			return newDemoFailure(
				demoErrorOperatorInstall,
				"The Qovery Operator Helm installation failed.",
				errors.New("operator Helm installation failed; sensitive bootstrap values were redacted"),
			)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := o.runPhase(ctx, demoPhaseOperatorHeartbeat, "Waiting for a fresh Qovery Operator heartbeat", func() error {
		return o.waitForOperator(ctx, cfg, clusterID)
	}); err != nil {
		return err
	}

	var status string
	if err := o.runPhase(ctx, demoPhasePlatformCatalogDeployment, "Deploying the current self-managed platform catalog", func() error {
		currentStatus, err := o.api.GetClusterStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot read cluster deployment status: %w", err))
		}
		deadline := o.clock.Now().Add(cfg.DeploymentTimeout)
		if isDemoDeploymentInProgress(currentStatus) {
			_, _ = fmt.Fprintf(o.out, "A cluster deployment is already in progress (%s); waiting for it to finish before deploying.\n", currentStatus)
			if err := o.waitUntilNoDeploymentInProgress(ctx, cfg, clusterID, deadline); err != nil {
				return err
			}
		}
		initialStatus, err := o.api.DeployCluster(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot trigger cluster deployment: %w", err))
		}
		status, err = o.waitForDeployment(ctx, cfg, clusterID, initialStatus, deadline)
		return err
	}); err != nil {
		return err
	}

	if err := o.runPhase(ctx, demoPhaseWorkloadVerification, "Verifying Operator and Engine workloads", func() error {
		if err := o.local.ValidateWorkloads(ctx, bootstrap.Namespace); err != nil {
			return newDemoFailure(demoErrorWorkloadVerification, "The Operator and Engine workloads are not healthy.", err)
		}
		return nil
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(
		o.out,
		"\nQovery demo cluster is now installed !!!!\nThe kubeconfig is correctly set, so you can connect to it directly with kubectl or k9s from your local machine.\nTo stop or start your cluster, use `k3d cluster stop|start %[1]s`. To delete it, use `qovery demo destroy -c %[1]s -d`.\n\nGo to https://console.qovery.com to create your first environment on this cluster %[1]q.\nCluster deployment finished with status %[2]s.\n",
		cfg.ClusterName,
		status,
	)
	return nil
}

func (o *demoOrchestrator) runPhase(ctx context.Context, phase string, message string, run func() error) error {
	o.phase(message)
	startedAt := o.clock.Now()
	err := ctx.Err()
	if err == nil {
		err = run()
	}
	duration := o.clock.Now().Sub(startedAt)
	if err == nil {
		_, _ = fmt.Fprintf(o.out, "✅ Done (%s)\n", duration.Round(time.Second))
	}
	result := demoPhaseResultSucceeded
	errorCode := ""
	safeErrorMessage := ""
	switch {
	case err != nil && ctx.Err() != nil:
		result = demoPhaseResultCanceled
	case err != nil:
		result = demoPhaseResultFailed
		errorCode, safeErrorMessage = demoFailureDetails(err)
	}
	if o.progress != nil {
		o.progress.PhaseFinished(demoPhaseExecution{
			Phase:            phase,
			Result:           result,
			Duration:         duration,
			ErrorCode:        errorCode,
			SafeErrorMessage: safeErrorMessage,
		})
	}
	return err
}

func (o *demoOrchestrator) clusterResolved(clusterID string) {
	if o.progress != nil {
		o.progress.ClusterResolved(clusterID)
	}
}

func (o *demoOrchestrator) waitForOperator(ctx context.Context, cfg demoConfig, clusterID string) error {
	deadline := o.clock.Now().Add(cfg.OperatorTimeout)
	for {
		status, err := o.api.GetOperatorStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot read Qovery Operator status: %w", err))
		}
		if operatorStatusReady(status, o.clock.Now()) {
			return nil
		}
		if !o.clock.Now().Before(deadline) {
			return newDemoFailure(
				demoErrorOperatorHeartbeatTimeout,
				"The Qovery Operator did not report a fresh heartbeat in time.",
				fmt.Errorf("timed out after %s waiting for a connected Qovery Operator with a fresh heartbeat", cfg.OperatorTimeout),
			)
		}
		if err := o.clock.Sleep(ctx, cfg.PollInterval); err != nil {
			return err
		}
	}
}

func operatorStatusReady(status demoOperatorStatus, now time.Time) bool {
	if !status.Connected || status.LastHeartbeat == nil {
		return false
	}
	return now.Sub(*status.LastHeartbeat) <= demoOperatorHeartbeatFreshness
}

func (o *demoOrchestrator) waitForDeployment(ctx context.Context, cfg demoConfig, clusterID string, initialStatus string, deadline time.Time) (string, error) {
	status := initialStatus
	for {
		if status == string(qovery.CLUSTERSTATEENUM_DEPLOYED) || status == string(qovery.CLUSTERSTATEENUM_RESTARTED) {
			return status, nil
		}
		if isDemoDeploymentError(status) {
			return "", newDemoFailure(
				demoErrorClusterDeployment,
				fmt.Sprintf("The cluster deployment finished with status %s.", status),
				fmt.Errorf("cluster deployment finished with terminal status %s", status),
			)
		}
		if !o.clock.Now().Before(deadline) {
			return "", newDemoFailure(
				demoErrorClusterDeploymentTimeout,
				"The cluster deployment did not finish in time.",
				fmt.Errorf("timed out after %s waiting for cluster deployment; last status was %s", cfg.DeploymentTimeout, status),
			)
		}
		if err := o.clock.Sleep(ctx, cfg.PollInterval); err != nil {
			return "", err
		}
		var err error
		status, err = o.api.GetClusterStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return "", demoQoveryAPIFailure(fmt.Errorf("cannot read cluster deployment status: %w", err))
		}
	}
}

func (o *demoOrchestrator) waitUntilNoDeploymentInProgress(ctx context.Context, cfg demoConfig, clusterID string, deadline time.Time) error {
	for {
		if !o.clock.Now().Before(deadline) {
			return newDemoFailure(
				demoErrorClusterDeploymentTimeout,
				"The cluster deployment did not finish in time.",
				fmt.Errorf("timed out after %s waiting for the cluster deployment already in progress", cfg.DeploymentTimeout),
			)
		}
		if err := o.clock.Sleep(ctx, cfg.PollInterval); err != nil {
			return err
		}
		status, err := o.api.GetClusterStatus(ctx, cfg.OrganizationID, clusterID)
		if err != nil {
			return demoQoveryAPIFailure(fmt.Errorf("cannot read cluster deployment status: %w", err))
		}
		if !isDemoDeploymentInProgress(status) {
			return nil
		}
	}
}

func isDemoDeploymentInProgress(status string) bool {
	return strings.HasSuffix(status, "QUEUED") || strings.HasSuffix(status, "ING")
}

func isDemoDeploymentError(status string) bool {
	switch qovery.ClusterStateEnum(status) {
	case qovery.CLUSTERSTATEENUM_INVALID_CREDENTIALS, qovery.CLUSTERSTATEENUM_CANCELED, qovery.CLUSTERSTATEENUM_STOPPED, qovery.CLUSTERSTATEENUM_DELETED:
		return true
	default:
		return strings.HasSuffix(status, "_ERROR")
	}
}

func (o *demoOrchestrator) phase(message string) {
	const separator = `""""""""""""""""""""""""""""""""""""""""""""`
	_, _ = fmt.Fprintf(o.out, "\n%s\n%s\n%s\n", separator, message, separator)
}
