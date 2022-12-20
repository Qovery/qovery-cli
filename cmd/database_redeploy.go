package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var databaseRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a database",
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

		_, _, err = client.DatabaseActionsApi.RestartDatabase(context.Background(), database.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Database is redeploying!")

		if watchFlag {
			utils.WatchDatabase(database.Id, client)
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseRedeployCmd)
	databaseRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseRedeployCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseRedeployCmd.Flags().StringVarP(&databaseCommitId, "commit-id", "c", "", "Database Commit ID")
	databaseRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")

	_ = databaseRedeployCmd.MarkFlagRequired("database")
}
