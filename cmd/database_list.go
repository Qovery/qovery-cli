package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"
	"strconv"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
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

		databases, _, err := client.DatabasesApi.ListDatabase(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getDatabaseJsonOutput(*client, statuses.GetDatabases(), databases.GetResults()))
			return
		}

		var data [][]string

		for _, database := range databases.GetResults() {
			res, _, err := client.DatabaseMainCallsApi.GetDatabaseMasterCredentials(context.Background(), database.Id).Execute()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			login := "********"
			password := "********"

			if showCredentials {
				login = res.Login

				if login == "" {
					login = "N/A"
				}

				password = res.Password
			}

			data = append(data, []string{database.Id, database.Name, "Database",
				utils.FindStatusTextWithColor(statuses.GetDatabases(), database.Id), res.Host, strconv.Itoa(int(res.Port)), login, password, database.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Host", "Port", "Login", "Password", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getDatabaseJsonOutput(client qovery.APIClient, statuses []qovery.Status, databases []qovery.Database) string {
	var results []interface{}

	for _, database := range databases {
		res, _, err := client.DatabaseMainCallsApi.GetDatabaseMasterCredentials(context.Background(), database.Id).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		results = append(results, map[string]interface{}{
			"id":            database.Id,
			"updated_at":    utils.ToIso8601(database.UpdatedAt),
			"name":          database.Name,
			"type":          "Database",
			"database_type": database.Type,
			"status":        utils.FindStatus(statuses, database.Id),
			"host":          database.Host,
			"port":          res.Port,
			"login":         res.Login,
			"password":      res.Password,
		})
	}

	j, err := json.Marshal(results)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(j)
}

func init() {
	databaseCmd.AddCommand(databaseListCmd)
	databaseListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseListCmd.Flags().BoolVarP(&showCredentials, "show-credentials", "", false, "Show Credentials")
	databaseListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
