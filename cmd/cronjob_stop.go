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

var cronjobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)
		validateCronjobArguments(cronjobName, cronjobNames)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		if isDeploymentQueueEnabledForOrganization(organizationId) {
			serviceIds := utils.Map(buildCronJobListFromCronjobNames(client, envId, cronjobName, cronjobNames),
				func(cronjob *qovery.JobResponse) string {
					return utils.GetJobId(cronjob)
				})
			_, err := client.EnvironmentActionsAPI.
				StopSelectedServices(context.Background(), envId).
				EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
					JobIds: serviceIds,
				}).
				Execute()
			checkError(err)
			utils.Println(fmt.Sprintf("Request to stop cronjob(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", cronjobName, cronjobNames)))
			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}
			return
		}
		// TODO(ENG-1883) once deployment queue is enabled for all organizations, remove the following code block

		if cronjobNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			cronjobs, err := ListCronjobs(envId, client)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, cronjobName := range strings.Split(cronjobNames, ",") {
				trimmedCronjobName := strings.TrimSpace(cronjobName)
				serviceIds = append(serviceIds, utils.FindByJobName(cronjobs, trimmedCronjobName).CronJobResponse.Id)
			}

			// stop multiple services
			_, err = utils.StopServices(client, envId, serviceIds, utils.JobType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Stopping cronjobs %s in progress..", pterm.FgBlue.Sprintf("%s", cronjobNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		cronjobs, err := ListCronjobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs, cronjobName)

		if cronjob == nil || cronjob.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, cronjob.CronJobResponse.Id, utils.JobType, watchFlag)

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
			utils.Println(fmt.Sprintf("Cronjob %s stopped!", pterm.FgBlue.Sprintf("%s", cronjobName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping cronjob %s in progress..", pterm.FgBlue.Sprintf("%s", cronjobName)))
		}
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
