package cmd

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.ShowHelpIfNoArgs(cmd, args)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateDatabaseArguments(databaseName, databaseNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		databaseList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)
		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				DatabaseIds: utils.Map(databaseList, func(database *qovery.Database) string {
					return database.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete database(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
		WatchDatabaseDeployment(client, envId, databaseList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	databaseCmd.AddCommand(databaseDeleteCmd)
	databaseDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseDeleteCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseDeleteCmd.Flags().StringVarP(&databaseNames, "databases", "", "", "Database Names (comma separated) Example: --databases \"db1,db2\"")
	databaseDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")
}
