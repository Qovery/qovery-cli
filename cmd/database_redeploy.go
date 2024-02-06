package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
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
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), envId).Execute()

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

		msg, err := utils.RedeployService(client, envId, database.Id, database.Name, utils.DatabaseType, watchFlag)

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
			utils.Println(fmt.Sprintf("Database %s redeployed!", pterm.FgBlue.Sprintf(databaseName)))
		} else {
			utils.Println(fmt.Sprintf("Redeploying database %s in progress..", pterm.FgBlue.Sprintf(databaseName)))
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseRedeployCmd)
	databaseRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseRedeployCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")

	_ = databaseRedeployCmd.MarkFlagRequired("database")
}
