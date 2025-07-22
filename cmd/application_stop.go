package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var applicationStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.ShowHelpIfNoArgs(cmd, args)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateApplicationArguments(applicationName, applicationNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		applicationList := buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)
		serviceIds := utils.Map(applicationList, func(application *qovery.Application) string {
			return application.Id
		})
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ApplicationIds: serviceIds,
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		WatchApplicationDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildApplicationListFromApplicationNames(
	client *qovery.APIClient,
	environmentId string,
	applicationName string,
	applicationNames string,
) []*qovery.Application {
	var applicationList []*qovery.Application
	applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), environmentId).Execute()
	checkError(err)

	if applicationName != "" {
		application := utils.FindByApplicationName(applications.GetResults(), applicationName)
		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		applicationList = append(applicationList, application)
	}
	if applicationNames != "" {
		for _, applicationName := range strings.Split(applicationNames, ",") {
			trimmedApplicationName := strings.TrimSpace(applicationName)
			application := utils.FindByApplicationName(applications.GetResults(), trimmedApplicationName)
			if application == nil {
				utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
				utils.PrintlnInfo("You can list all applications with: qovery application list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			applicationList = append(applicationList, application)
		}
	}

	return applicationList
}

func validateApplicationArguments(applicationName string, applicationNames string) {
	if applicationName == "" && applicationNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --application \"<app name>\" or --applications \"<app1 name>, <app2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if applicationName != "" && applicationNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --application and --applications at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func checkError(err error) {
	utils.CheckError(err)
}

func init() {
	applicationCmd.AddCommand(applicationStopCmd)
	applicationStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationStopCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationStopCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2,app3\"")
	applicationStopCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
