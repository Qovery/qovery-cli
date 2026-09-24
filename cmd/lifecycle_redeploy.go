package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateLifecycleArguments(lifecycleName, lifecycleNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		lifecycleList := buildLifecycleListFromLifecycleNames(client, envId, lifecycleName, lifecycleNames)
		_, _, err := client.JobActionsAPI.
			DeployJob(context.Background(), utils.GetJobId(lifecycleList[0])).
			JobDeployRequest(qovery.JobDeployRequest{}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy lifecycle job(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", lifecycleName, lifecycleNames)))
		WatchJobDeployment(client, envId, lifecycleList, watchFlag, qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleRedeployCmd)
	lifecycleRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleRedeployCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")

	_ = lifecycleRedeployCmd.MarkFlagRequired("lifecycle")
}
