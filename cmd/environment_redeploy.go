package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"time"
)

var environmentRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)
		_, _, err := client.EnvironmentActionsAPI.
			DeployEnvironment(context.Background(), envId).
			Execute()
		checkError(err)
		utils.Println("Request to redeploy environment has been queued..")
		if watchFlag {
			time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchEnvironment(envId, qovery.STATEENUM_DEPLOYED, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentRedeployCmd)
	environmentRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
