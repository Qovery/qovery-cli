package cmd

import (
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
		watch := utils.NewServicesWatch(client, envId, applicationIds(applicationList), watchFlag)
		err := utils.RebootServices(client, envId, qovery.RebootServicesRequest{ApplicationIds: applicationIds(applicationList)})
		checkError(err)
		utils.Println(fmt.Sprintf("Request to restart application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		watch.Wait(qovery.STATEENUM_RESTARTED)
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
