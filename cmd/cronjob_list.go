package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var cronjobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cronjobs",
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

		cronjobs, err := ListCronjobs(envId, client)

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

		for _, cronjob := range cronjobs {
			data = append(data, []string{cronjob.Name, "Cronjob",
				utils.GetStatus(statuses.GetJobs(), cronjob.Id), cronjob.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobListCmd)
	cronjobListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
