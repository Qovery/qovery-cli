package cmd

import (
	"errors"
	"fmt"
	"strings"
)

const (
	demoErrorUnsupportedOS            = "UNSUPPORTED_OS"
	demoErrorAuthentication           = "AUTHENTICATION_FAILED"
	demoErrorOrganization             = "ORGANIZATION_NOT_RESOLVED"
	demoErrorInvalidClusterName       = "INVALID_CLUSTER_NAME"
	demoErrorMissingDependency        = "MISSING_DEPENDENCY"
	demoErrorDockerNotRunning         = "DOCKER_NOT_RUNNING"
	demoErrorQoveryAPI                = "QOVERY_API_ERROR"
	demoErrorClusterNameNotADemo      = "CLUSTER_NAME_NOT_A_DEMO"
	demoErrorK3dCluster               = "K3D_CLUSTER_FAILED"
	demoErrorSubnetInUse              = "DEMO_SUBNET_IN_USE"
	demoErrorLoopback                 = "LOOPBACK_SETUP_FAILED"
	demoErrorHelmList                 = "HELM_LIST_FAILED"
	demoErrorLegacyRelease            = "LEGACY_RELEASE_EXISTS"
	demoErrorOperatorInstall          = "OPERATOR_INSTALL_FAILED"
	demoErrorOperatorHeartbeatTimeout = "OPERATOR_HEARTBEAT_TIMEOUT"
	demoErrorClusterDeployment        = "CLUSTER_DEPLOYMENT_FAILED"
	demoErrorClusterDeploymentTimeout = "CLUSTER_DEPLOYMENT_TIMEOUT"
	demoErrorWorkloadVerification     = "WORKLOAD_VERIFICATION_FAILED"
)

type demoFailure struct {
	code        string
	safeMessage string
	err         error
}

func (f *demoFailure) Error() string { return f.err.Error() }

func (f *demoFailure) Unwrap() error { return f.err }

func newDemoFailure(code string, safeMessage string, err error) error {
	return &demoFailure{code: code, safeMessage: safeMessage, err: err}
}

func demoQoveryAPIFailure(err error) error {
	return newDemoFailure(demoErrorQoveryAPI, "A Qovery API request failed.", err)
}

func demoFailureDetails(err error) (string, string) {
	var failure *demoFailure
	if errors.As(err, &failure) {
		return failure.code, failure.safeMessage
	}
	return demoUnknownErrorCode, demoSafeErrorMessage
}

func demoSubnetInUseError(networkName string) error {
	hint := fmt.Sprintf("Remove this network with `docker network rm %s`, then run `qovery demo up` again.", networkName)
	if clusterName, isK3d := strings.CutPrefix(networkName, "k3d-"); isK3d {
		hint = fmt.Sprintf(
			"k3d created it for the cluster %q. If it is an old demo cluster, delete it with `k3d cluster delete %s`. If that cluster no longer exists, remove the network with `docker network rm %s`. Then run `qovery demo up` again.",
			clusterName, clusterName, networkName,
		)
	}
	return newDemoFailure(
		demoErrorSubnetInUse,
		fmt.Sprintf("Another Docker network already uses the demo subnet %s.", demoSubnet),
		fmt.Errorf("the Docker network %q already uses the demo subnet %s\n%s", networkName, demoSubnet, hint),
	)
}

var demoDependencyInstallDocs = map[string]string{
	"docker":  "https://docs.docker.com/get-started/get-docker/",
	"k3d":     "https://k3d.io/stable/#installation",
	"helm":    "https://helm.sh/docs/intro/install/",
	"kubectl": "https://kubernetes.io/docs/tasks/tools/",
}

func missingDemoDependenciesError(goos string, missing []string) error {
	var message strings.Builder
	fmt.Fprintf(&message, "required commands are not installed: %s\nInstall them, then run `qovery demo up` again:", strings.Join(missing, ", "))
	var brewInstallable []string
	for _, dependency := range missing {
		if dependency != "docker" {
			brewInstallable = append(brewInstallable, dependency)
		}
	}
	if goos == "darwin" && len(brewInstallable) > 0 {
		fmt.Fprintf(&message, "\n  brew install %s", strings.Join(brewInstallable, " "))
	}
	for _, dependency := range missing {
		fmt.Fprintf(&message, "\n  %s: %s", dependency, demoDependencyInstallDocs[dependency])
	}
	return newDemoFailure(
		demoErrorMissingDependency,
		fmt.Sprintf("Required commands are not installed: %s.", strings.Join(missing, ", ")),
		errors.New(message.String()),
	)
}
