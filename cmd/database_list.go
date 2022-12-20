package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
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

		databases, _, err := client.DatabasesApi.ListDatabase(context.Background(), envId).Execute()

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

		for _, database := range databases.GetResults() {
			data = append(data, []string{database.Name, "Database",
				utils.GetStatus(statuses.GetDatabases(), database.Id), database.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseListCmd)
	databaseListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
