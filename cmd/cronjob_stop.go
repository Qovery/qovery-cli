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

var cronjobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateCronjobArguments(cronjobName, cronjobNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		cronJobList := buildCronJobListFromCronjobNames(client, envId, cronjobName, cronjobNames)
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				JobIds: utils.Map(cronJobList, func(job *qovery.JobResponse) string {
					return utils.GetJobId(job)
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop cronjob(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", cronjobName, cronjobNames)))
		WatchJobDeployment(client, envId, cronJobList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildCronJobListFromCronjobNames(
	client *qovery.APIClient,
	environmentId string,
	cronjobName string,
	cronjobNames string,
) []*qovery.JobResponse {
	var cronjobList []*qovery.JobResponse
	cronjobs, _, err := client.JobsAPI.ListJobs(context.Background(), environmentId).Execute()
	checkError(err)

	if cronjobName != "" {
		cronjob := utils.FindByJobName(cronjobs.GetResults(), cronjobName)
		if cronjob == nil || cronjob.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		cronjobList = append(cronjobList, cronjob)
	}
	if cronjobNames != "" {
		for _, cronjobName := range strings.Split(cronjobNames, ",") {
			trimmedCronjobName := strings.TrimSpace(cronjobName)
			cronjob := utils.FindByJobName(cronjobs.GetResults(), trimmedCronjobName)
			if cronjob == nil || cronjob.CronJobResponse == nil {
				utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
				utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			cronjobList = append(cronjobList, cronjob)
		}
	}

	return cronjobList
}

func validateCronjobArguments(cronJobName string, cronJobNames string) {
	if cronJobName == "" && cronJobNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --cronjob \"<cronjob name>\" or --cronjobs \"<cronjob1 name>, <cronjob2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if cronJobName != "" && cronJobNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --cronjob and --cronjobs at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func init() {
	cronjobCmd.AddCommand(cronjobStopCmd)
	cronjobStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobStopCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobStopCmd.Flags().StringVarP(&cronjobNames, "cronjobs", "", "", "Cronjob Names (comma separated) (ex: --cronjobs \"cron1,cron2\")")
	cronjobStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")
}
