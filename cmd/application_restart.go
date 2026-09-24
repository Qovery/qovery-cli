package cmd

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart an application without redeploying it",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		_, _, err := client.EnvironmentActionsAPI.
			RebootServices(context.Background(), envId).
			RebootServicesRequest(qovery.RebootServicesRequest{
				ApplicationIds: utils.Map(applicationList, func(application *qovery.Application) string {
					return application.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to restart application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		WatchApplicationDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	applicationCmd.AddCommand(applicationRestartCmd)
	applicationRestartCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationRestartCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationRestartCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationRestartCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationRestartCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2\"")
	applicationRestartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
