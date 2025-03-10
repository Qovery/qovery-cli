package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

var databaseStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateDatabaseArguments(databaseName, databaseNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		applicationList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				DatabaseIds: utils.Map(applicationList, func(database *qovery.Database) string {
					return database.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop databases %s has been queued...", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
		WatchDatabaseDeployment(client, envId, applicationList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildDatabaseListFromDatabaseNames(
	client *qovery.APIClient,
	environmentId string,
	databaseName string,
	databaseNames string,
) []*qovery.Database {
	var databaseList []*qovery.Database
	databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), environmentId).Execute()
	checkError(err)

	if databaseName != "" {
		database := utils.FindByDatabaseName(databases.GetResults(), databaseName)
		if database == nil {
			utils.PrintlnError(fmt.Errorf("database %s not found", databaseName))
			utils.PrintlnInfo("You can list all databases with: qovery database list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		databaseList = append(databaseList, database)
	}
	if databaseNames != "" {
		for _, databaseName := range strings.Split(databaseNames, ",") {
			trimmedDatabaseName := strings.TrimSpace(databaseName)
			database := utils.FindByDatabaseName(databases.GetResults(), trimmedDatabaseName)
			if database == nil {
				utils.PrintlnError(fmt.Errorf("database %s not found", databaseName))
				utils.PrintlnInfo("You can list all databases with: qovery database list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			databaseList = append(databaseList, database)
		}
	}

	return databaseList
}

func validateDatabaseArguments(databaseName string, databaseNames string) {
	if databaseName == "" && databaseNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --database \"<database name>\" or --databases \"<database1 name>, <database2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if databaseName != "" && databaseNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --database and --databases at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func init() {
	databaseCmd.AddCommand(databaseStopCmd)
	databaseStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseStopCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseStopCmd.Flags().StringVarP(&databaseNames, "databases", "", "", "Database Names (comma separated) Example: --databases \"db1,db2\"")
	databaseStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")
}
