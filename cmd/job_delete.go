package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var jobDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a job",
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

		_, err = client.JobMainCallsApi.DeleteJob(context.Background(), job.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Job is deleting!")

		if watchFlag {
			utils.WatchJob(job.Id, client)
		}
	},
}

func init() {
	jobCmd.AddCommand(jobDeleteCmd)
	jobDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	jobDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	jobDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	jobDeleteCmd.Flags().StringVarP(&jobName, "job", "n", "", "Job Name")
	jobDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch job status until it's ready or an error occurs")

	_ = jobDeleteCmd.MarkFlagRequired("job")
}
