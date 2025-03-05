package cmd

import (
	"context"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateCronjobArguments(cronjobName, cronjobNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		cronJobList := buildCronJobListFromCronjobNames(client, envId, cronjobName, cronjobNames)
		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				JobIds: utils.Map(cronJobList, func(job *qovery.JobResponse) string {
					return utils.GetJobId(job)
				}),
			}).
			Execute()
		checkError(err)
		WatchJobDeployment(client, envId, cronJobList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobDeleteCmd)
	cronjobDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobDeleteCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobDeleteCmd.Flags().StringVarP(&cronjobNames, "cronjobs", "", "", "Cronjob Names (comma separated) (ex: --cronjobs \"cron1,cron2\")")
	cronjobDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")
}
