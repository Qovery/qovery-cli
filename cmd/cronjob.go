package cmd

import (
	"context"
	"os"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobName string
var cronjobNames string
var cronjobCommitId string
var cronjobBranch string
var cronjobTag string
var cronjobImageName string

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
	jobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

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
