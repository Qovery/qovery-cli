package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"time"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)
		validateApplicationArguments(applicationName, applicationNames)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		// deploy multiple services
		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		err = utils.DeployApplications(client, envId, applicationList, applicationCommitId)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy application(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		if watchFlag {
			time.Sleep(5 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchEnvironment(envId, "unused", client)
		}
		return
	},
}

func init() {
	applicationCmd.AddCommand(applicationDeployCmd)
	applicationDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDeployCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2,app3\"")
	applicationDeployCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
