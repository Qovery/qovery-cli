package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var cronjobName string
var cronjobCommitId string

var targetCronjobName string

var cronjobCmd = &cobra.Command{
	Use:   "cronjob",
	Short: "Manage cronjobs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(cronjobCmd)
}

func ListCronjobs(envId string, client *qovery.APIClient) ([]qovery.JobResponse, error) {
	jobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

	if err != nil {
		return nil, err
	}

	cronjobs := make([]qovery.JobResponse, 0)
	for _, job := range jobs.GetResults() {
		schedule := job.GetSchedule()
		cronjob, _ := schedule.GetCronjobOk()

		if cronjob != nil && cronjob.ScheduledAt != "" {
			cronjobs = append(cronjobs, job)
		}
	}

	return cronjobs, nil
}
