package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"time"

	"github.com/qovery/qovery-cli/utils"
)

var applicationRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		application := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)[0]

		deployRequest := qovery.DeployRequest{GitCommitId: *application.GitRepository.DeployedCommitId}

		_, _, err = client.ApplicationActionsAPI.DeployApplication(context.Background(), application.Id).
			DeployRequest(deployRequest).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy application(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))

		if watchFlag {
			time.Sleep(5 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchApplication(application.Id, envId, client)
		}
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
