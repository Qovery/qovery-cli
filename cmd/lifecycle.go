package cmd

import (
	"context"
	"os"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleName string
var lifecycleNames string
var lifecycleCommitId string
var lifecycleTag string
var lifecycleImageName string
var lifecycleBranch string
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
	jobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

	if err != nil {
		return nil, err
	}

	lifecycleJobs := make([]qovery.JobResponse, 0)
	for _, job := range jobs.GetResults() {
		if job.LifecycleJobResponse != nil {
			lifecycleJobs = append(lifecycleJobs, job)
		}
	}

	return lifecycleJobs, nil
}
