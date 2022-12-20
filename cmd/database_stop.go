package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var databaseStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a database",
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

		database := utils.FindByDatabaseName(databases.GetResults(), databaseName)

		if database == nil {
			utils.PrintlnError(fmt.Errorf("database %s not found", databaseName))
			utils.PrintlnInfo("You can list all databases with: qovery database list")
			os.Exit(1)
		}

		_, _, err = client.DatabaseActionsApi.StopDatabase(context.Background(), database.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Database is stopping!")

		if watchFlag {
			utils.WatchDatabase(database.Id, client)
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseStopCmd)
	databaseStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseStopCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")

	_ = databaseStopCmd.MarkFlagRequired("database")
}
