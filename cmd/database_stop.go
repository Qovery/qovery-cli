package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		validateDatabaseArguments(databaseName, databaseNames)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		checkError(err)

		if isDeploymentQueueEnabledForOrganization(organizationId) {
			serviceIds := utils.Map(buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames),
				func(database *qovery.Database) string {
					return database.Id
				})
			_, err := client.EnvironmentActionsAPI.
				StopSelectedServices(context.Background(), envId).
				EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
					DatabaseIds: serviceIds,
				}).
				Execute()
			checkError(err)
			utils.Println(fmt.Sprintf("Request to stop databases %s has been queued...", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}
			return
		}

		// TODO(ENG-1883) once deployment queue is enabled for all organizations, remove the following code block

		if databaseNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), envId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, databaseName := range strings.Split(databaseNames, ",") {
				trimmedDatabaseName := strings.TrimSpace(databaseName)
				serviceIds = append(serviceIds, utils.FindByDatabaseName(databases.GetResults(), trimmedDatabaseName).Id)
			}

			// stop multiple services
			_, err = utils.StopServices(client, envId, serviceIds, utils.DatabaseType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Stopping databases %s in progress..", pterm.FgBlue.Sprintf("%s", databaseNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
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
			utils.Println(fmt.Sprintf("Database %s stopped!", pterm.FgBlue.Sprintf("%s", databaseName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping database %s in progress..", pterm.FgBlue.Sprintf("%s", databaseName)))
		}
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
