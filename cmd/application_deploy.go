package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// deploy multiple services
		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		watch := utils.NewServicesWatch(client, envId, applicationIds(applicationList), watchFlag)
		err := utils.DeployApplications(client, envId, applicationList, applicationCommitID)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy application(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	applicationCmd.AddCommand(applicationDeployCmd)
	applicationDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDeployCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2,app3\"")
	applicationDeployCmd.Flags().StringVarP(&applicationCommitID, "commit-id", "c", "", "Application Commit ID, or 'latest' for the newest commit of the branch")
	applicationDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
