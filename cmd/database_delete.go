package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
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
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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
			_, err = utils.DeleteServices(client, envId, serviceIds, utils.DatabaseType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Deleting databases %s in progress..", pterm.FgBlue.Sprintf("%s", databaseNames)))
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

		msg, err := utils.DeleteService(client, envId, database.Id, utils.DatabaseType, watchFlag)

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
			utils.Println(fmt.Sprintf("Database %s deleted!", pterm.FgBlue.Sprintf("%s", databaseName)))
		} else {
			utils.Println(fmt.Sprintf("Deleting database %s in progress..", pterm.FgBlue.Sprintf("%s", databaseName)))
		}
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
