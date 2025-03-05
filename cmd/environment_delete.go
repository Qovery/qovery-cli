package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"time"
)

var environmentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		_, err := client.EnvironmentMainCallsAPI.
			DeleteEnvironment(context.Background(), envId).
			Execute()
		checkError(err)
		utils.Println("Request to delete environment has been queued...")
		if watchFlag {
			time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent
			utils.WatchEnvironment(envId, qovery.STATEENUM_DELETED, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentDeleteCmd)
	environmentDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
