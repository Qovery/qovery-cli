package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateLifecycleArguments(lifecycleName, lifecycleNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		lifecycleList := buildLifecycleListFromLifecycleNames(client, envId, lifecycleName, lifecycleNames)

		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				JobIds: utils.Map(lifecycleList, func(lifecycle *qovery.JobResponse) string {
					return utils.GetJobId(lifecycle)
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete lifecycle job(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", lifecycleName, lifecycleNames)))
		WatchJobDeployment(client, envId, lifecycleList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeleteCmd)
	lifecycleDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeleteCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeleteCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle Job Names (comma separated) (ex: --lifecycles \"lifecycle1,lifecycle2\")")
	lifecycleDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle job status until it's ready or an error occurs")
}
