package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	adminForceFailedDeploymentsToInternalErrorCmd = &cobra.Command{
		Use:   "force-failed-deployments-to-internal-error",
		Short: "Force the status of environment deployments in a non-final state to INTERNAL_ERROR, and also force any of the deployment statuses associated",
		Run: func(cmd *cobra.Command, args []string) {
			forceFailedDeploymentsToInternalErrorStatus()
		},
	}
)

func init() {
	adminCmd.AddCommand(adminForceFailedDeploymentsToInternalErrorCmd)
}

func forceFailedDeploymentsToInternalErrorStatus() {
	pkg.ForceFailedDeploymentsToInternalErrorStatus()
}
