package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var applicationRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)

		_, _, err := client.ApplicationActionsAPI.DeployApplication(context.Background(), applicationList[0].Id).
			DeployRequest(qovery.DeployRequest{GitCommitId: *applicationList[0].GitRepository.DeployedCommitId}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy application(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		WatchApplicationDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	applicationCmd.AddCommand(applicationRedeployCmd)
	applicationRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationRedeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationRedeployCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")

	_ = applicationRedeployCmd.MarkFlagRequired("application")
}
