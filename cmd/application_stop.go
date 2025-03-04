package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
)

var applicationStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)
		validateApplicationArguments(applicationName, applicationNames)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		serviceIds := buildServiceIdsFromApplicationNames(client, envId, applicationName, applicationNames)
		_, err = client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ApplicationIds: serviceIds,
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop application(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", applicationName, applicationNames)))
		if watchFlag {
			time.Sleep(5 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			utils.WatchEnvironment(envId, "unused", client)
		}
		return
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

func buildServiceIdsFromApplicationNames(
	client *qovery.APIClient,
	environmentId string,
	applicationName string,
	applicationNames string,
) []string {
	applicationList := buildApplicationListFromApplicationNames(client, environmentId, applicationName, applicationNames)
	serviceIds := make([]string, len(applicationList))

	for i, item := range applicationList {
		serviceIds[i] = item.Id
	}
	return serviceIds
}

func isDeploymentQueueEnabledForOrganization(organizationId string) bool {
	return organizationId == "3f421018-8edf-4a41-bb86-bec62791b6dc" || // backdev
		organizationId == "3d542888-3d2c-474a-b1ad-712556db66da" // QSandbox
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
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
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
