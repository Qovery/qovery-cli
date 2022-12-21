package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var databaseDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a database",
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

		_, err = client.DatabaseMainCallsApi.DeleteDatabase(context.Background(), database.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println(fmt.Sprintf("Deleting database %s in progress..", pterm.FgBlue.Sprintf(databaseName)))

		if watchFlag {
			utils.WatchDatabase(database.Id, envId, client)
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseDeleteCmd)
	databaseDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseDeleteCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")

	_ = databaseDeleteCmd.MarkFlagRequired("database")
}
