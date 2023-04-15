package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
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
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		databases, _, err := client.DatabasesApi.ListDatabase(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		database := utils.FindByDatabaseName(databases.GetResults(), databaseName)

		if database == nil {
			utils.PrintlnError(fmt.Errorf("database %s not found", databaseName))
			utils.PrintlnInfo("You can list all databases with: qovery database list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, database.Id, utils.DatabaseType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Database %s stopped!", pterm.FgBlue.Sprintf(databaseName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping database %s in progress..", pterm.FgBlue.Sprintf(databaseName)))
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
