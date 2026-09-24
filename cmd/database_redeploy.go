package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateDatabaseArguments(databaseName, databaseNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		databaseList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)
		_, _, err := client.DatabaseActionsAPI.
			DeployDatabase(context.Background(), databaseList[0].Id).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy database(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
		WatchDatabaseDeployment(client, envId, databaseList, watchFlag, qovery.STATEENUM_RESTARTED)
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
