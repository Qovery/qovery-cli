package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var lifecycleName string
var lifecycleCommitId string
var targetLifecycleName string

var lifecycleCmd = &cobra.Command{
	Use:   "lifecycle",
	Short: "Manage lifecycle jobs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(lifecycleCmd)
}

func ListLifecycleJobs(envId string, client *qovery.APIClient) ([]qovery.JobResponse, error) {
	jobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

	if err != nil {
		return nil, err
	}

	cronjobs := make([]qovery.JobResponse, 0)
	for _, job := range jobs.GetResults() {
		schedule := job.GetSchedule()
		cronjob, _ := schedule.GetCronjobOk()

		if cronjob == nil || cronjob.ScheduledAt == "" {
			cronjobs = append(cronjobs, job)
		}
	}

	return cronjobs, nil
}
