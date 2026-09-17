package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"time"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	adminForceFailedDeploymentsToInternalErrorCmd = &cobra.Command{
		Use:   "force-failed-deployments-to-internal-error",
		Short: "Force the status of environment deployments in a non-final state to INTERNAL_ERROR, and also force any of the deployment statuses associated",
		Run: func(cmd *cobra.Command, args []string) {
			safeDuration, _ := cmd.Flags().GetString("safeguardDuration")
			duration, err := time.ParseDuration(safeDuration)
			if err != nil {
				log.Errorf("Could not parse duration : %s. Got %s", err, safeDuration)
				os.Exit(1)
			}
			forceFailedDeploymentsToInternalErrorStatus(duration)
		},
	}
)

func init() {
	adminForceFailedDeploymentsToInternalErrorCmd.Flags().StringP("safeguardDuration", "d", "20m", "wait at least the duration for env in non final state that haven't been updated to mark them as failed")
	adminCmd.AddCommand(adminForceFailedDeploymentsToInternalErrorCmd)
}

func forceFailedDeploymentsToInternalErrorStatus(duration time.Duration) {
	pkg.ForceFailedDeploymentsToInternalErrorStatus(duration)
}
