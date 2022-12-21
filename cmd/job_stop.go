package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var jobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a job",
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

		if !utils.IsEnvironmentInATerminalState(envId, client) {
			utils.PrintlnError(fmt.Errorf("environment id '%s' is not in a terminal state. The request is not queued and you must wait "+
				"for the end of the current operation to run your command. Try again in a few moment", envId))
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

		_, _, err = client.JobActionsApi.StopJob(context.Background(), job.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Job is stopping!")

		if watchFlag {
			utils.WatchJob(job.Id, envId, client)
		}
	},
}

func init() {
	jobCmd.AddCommand(jobStopCmd)
	jobStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	jobStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	jobStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	jobStopCmd.Flags().StringVarP(&jobName, "job", "n", "", "Job Name")
	jobStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch job status until it's ready or an error occurs")

	_ = jobStopCmd.MarkFlagRequired("job")
}
