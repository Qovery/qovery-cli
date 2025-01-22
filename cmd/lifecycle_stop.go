package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		validateLifecycleArguments(lifecycleName, lifecycleNames)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		if isDeploymentQueueEnabledForOrganization(organizationId) {
			serviceIds := buildServiceIdsFromLifecycleNames(client, envId, lifecycleName, lifecycleNames)
			_, err := client.EnvironmentActionsAPI.
				StopSelectedServices(context.Background(), envId).
				EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
					JobIds: serviceIds,
				}).
				Execute()
			checkError(err)
			utils.Println(fmt.Sprintf("Request to stop lifecyclejob(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", lifecycleName, lifecycleNames)))
			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}
			return
		}

		// TODO(ENG-1883) once deployment queue is enabled for all organizations, remove the following code block

		if lifecycleNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			lifecycles, err := ListLifecycleJobs(envId, client)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, lifecycleName := range strings.Split(lifecycleNames, ",") {
				trimmedLifecycleName := strings.TrimSpace(lifecycleName)
				serviceIds = append(serviceIds, utils.GetJobId(utils.FindByJobName(lifecycles, trimmedLifecycleName)))
			}

			// stop multiple services
			_, err = utils.StopServices(client, envId, serviceIds, utils.JobType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Stopping lifecycle jobs %s in progress..", pterm.FgBlue.Sprintf("%s", lifecycleNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, lifecycle.LifecycleJobResponse.Id, utils.JobType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Lifecycle %s stopped!", pterm.FgBlue.Sprintf("%s", lifecycleName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping lifecycle %s in progress..", pterm.FgBlue.Sprintf("%s", lifecycleName)))
		}
	},
}

func buildServiceIdsFromLifecycleNames(
	client *qovery.APIClient,
	environmentId string,
	lifecycleName string,
	lifecycleNames string,
) []string {
	var serviceIds []string
	lifecycles, _, err := client.JobsAPI.ListJobs(context.Background(), environmentId).Execute()
	checkError(err)

	if lifecycleName != "" {
		lifecycle := utils.FindByJobName(lifecycles.GetResults(), lifecycleName)
		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		serviceIds = append(serviceIds, lifecycle.LifecycleJobResponse.Id)
	}
	if lifecycleNames != "" {
		for _, lifecycleName := range strings.Split(lifecycleNames, ",") {
			trimmedLifecycleName := strings.TrimSpace(lifecycleName)
			lifecycle := utils.FindByJobName(lifecycles.GetResults(), trimmedLifecycleName)
			if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
				utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
				utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			serviceIds = append(serviceIds, lifecycle.LifecycleJobResponse.Id)
		}
	}

	return serviceIds
}

func validateLifecycleArguments(lifecycleName string, lifecycleNames string) {
	if lifecycleName == "" && lifecycleNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --lifecycle \"<lifecycle name>\" or --lifecycles \"<lifecycle1 name>, <lifecycle2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if lifecycleName != "" && lifecycleNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --lifecycle and --lifecycles at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func init() {
	lifecycleCmd.AddCommand(lifecycleStopCmd)
	lifecycleStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleStopCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleStopCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle Job Names (comma separated) (ex: --lifecycles \"lifecycle1,lifecycle2\")")
	lifecycleStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")
}
