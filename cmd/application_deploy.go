package cmd

import (
	"fmt"
	"time"

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
		utils.ShowHelpIfNoArgs(cmd, args)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// deploy multiple services
		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		err := utils.DeployApplications(client, envId, applicationList, applicationCommitId)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy application(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		WatchApplicationDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func WatchApplicationDeployment(
	client *qovery.APIClient,
	envId string,
	applications []*qovery.Application,
	watchFlag bool,
	finalServiceState qovery.StateEnum,
) {
	if watchFlag {
		time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
		if len(applications) == 1 {
			utils.WatchApplication(applications[0].Id, envId, client)
		} else {
			utils.WatchEnvironment(envId, finalServiceState, client)
		}
	}
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
