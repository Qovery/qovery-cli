package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
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

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		var data [][]string

		for _, job := range jobs.GetResults() {
			data = append(data, []string{job.Name, "Job",
				utils.GetStatus(statuses.GetJobs(), job.Id), job.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	jobCmd.AddCommand(jobListCmd)
	jobListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	jobListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	jobListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
