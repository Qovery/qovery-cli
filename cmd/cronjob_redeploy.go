package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateCronjobArguments(cronjobName, cronjobNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)
		cronJobList := buildCronJobListFromCronjobNames(client, envId, cronjobName, cronjobNames)

		_, _, err := client.JobActionsAPI.DeployJob(context.Background(), utils.GetJobId(cronJobList[0])).
			JobDeployRequest(qovery.JobDeployRequest{}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy cronjob(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", cronjobName, cronjobNames)))
		WatchCronJobDeployment(client, envId, cronJobList, watchFlag, qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobRedeployCmd)
	cronjobRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobRedeployCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")

	_ = cronjobRedeployCmd.MarkFlagRequired("cronjob")
}
