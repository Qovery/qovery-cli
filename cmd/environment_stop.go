package cmd

import (
	"context"
	"time"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		_, _, err := client.EnvironmentActionsAPI.
			StopEnvironment(context.Background(), envId).
			Execute()
		checkError(err)
		utils.Println("Environment stop request has been queued..")

		if watchFlag {
			time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchEnvironment(envId, qovery.STATEENUM_STOPPED, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentStopCmd)
	environmentStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
