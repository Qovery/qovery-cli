package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var jobRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		jobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		job := utils.FindByJobName(jobs.GetResults(), jobName)

		if job == nil {
			utils.PrintlnError(fmt.Errorf("job %s not found", jobName))
			utils.PrintlnInfo("You can list all jobs with: qovery job list")
			os.Exit(1)
		}

		_, _, err = client.JobActionsApi.RestartJob(context.Background(), job.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Job is redeploying!")

		if watchFlag {
			utils.WatchJob(job.Id, client)
		}
	},
}

func init() {
	jobCmd.AddCommand(jobRedeployCmd)
	jobRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	jobRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	jobRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	jobRedeployCmd.Flags().StringVarP(&jobName, "job", "n", "", "Job Name")
	jobRedeployCmd.Flags().StringVarP(&jobCommitId, "commit-id", "c", "", "Job Commit ID")
	jobRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch job status until it's ready or an error occurs")

	_ = jobRedeployCmd.MarkFlagRequired("job")
}
