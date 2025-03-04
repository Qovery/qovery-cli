package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		validateApplicationArguments(applicationName, applicationNames)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		serviceIds := buildServiceIdsFromApplicationNames(client, envId, applicationName, applicationNames)
		// stop multiple services
		_, err = client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ApplicationIds: serviceIds,
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		if watchFlag {
			time.Sleep(5 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchEnvironment(envId, "unused", client)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationDeleteCmd)
	applicationDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDeleteCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDeleteCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2,app3\"")
	applicationDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
