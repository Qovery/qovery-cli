package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		serviceIds := utils.Map(applicationList, func(application *qovery.Application) string {
			return application.Id
		})
		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ApplicationIds: serviceIds,
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		WatchApplicationDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_DELETED)
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
