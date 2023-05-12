package cmd

import (
	"context"
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

			data = append(data, []string{database.Name, "Database",
				utils.GetStatus(statuses.GetDatabases(), database.Id), res.Host, strconv.Itoa(int(res.Port)), login, password, database.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "Host", "Port", "Login", "Password", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	databaseCmd.AddCommand(databaseListCmd)
	databaseListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseListCmd.Flags().BoolVarP(&showCredentials, "show-credentials", "", false, "Show Credentials")
}
