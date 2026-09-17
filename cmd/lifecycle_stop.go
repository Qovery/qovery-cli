package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateLifecycleArguments(lifecycleName, lifecycleNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		lifecycleList := buildLifecycleListFromLifecycleNames(client, envId, lifecycleName, lifecycleNames)
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				JobIds: utils.Map(lifecycleList, func(lifecycle *qovery.JobResponse) string {
					return utils.GetJobId(lifecycle)
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop lifecycle job(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", lifecycleName, lifecycleNames)))
		WatchJobDeployment(client, envId, lifecycleList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildLifecycleListFromLifecycleNames(
	client *qovery.APIClient,
	environmentId string,
	lifecycleName string,
	lifecycleNames string,
) []*qovery.JobResponse {
	var lifecycleList []*qovery.JobResponse
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
		lifecycleList = append(lifecycleList, lifecycle)
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
			lifecycleList = append(lifecycleList, lifecycle)
		}
	}

	return lifecycleList
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
