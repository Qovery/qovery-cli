package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"net/http"
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

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		blueprintDatabases, err := listBlueprintDatabases(client, envId, showCredentials)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		if jsonFlag {
			utils.Println(getDatabaseJsonOutput(*client, statuses, databases.GetResults(), blueprintDatabases))
			return
		}

		var data [][]string

		for _, database := range databases.GetResults() {
			host, port, credentials, err := nativeDatabaseConnection(client, database)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			login := "********"
			password := "********"

			if credentials != nil {
				login = credentials.Login

				if login == "" {
					login = "N/A"
				}

				password = credentials.Password
			}

			data = append(data, []string{database.Id, database.Name, "Database",
				utils.FindStatusTextWithColor(statuses.GetDatabases(), database.Id), host, strconv.Itoa(int(port)), login, password, database.UpdatedAt.String()})
		}

		for _, database := range blueprintDatabases {
			host, port, login, password := database.connection()

			if !showCredentials {
				login = "********"
				password = "********"
			}

			data = append(data, []string{database.terraform.Id, database.terraform.Name, "Blueprint database",
				utils.FindStatusTextWithColor(statuses.GetTerraforms(), database.terraform.Id), host, port, login, password, database.terraform.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Host", "Port", "Login", "Password", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func getDatabaseJsonOutput(client qovery.APIClient, statuses *qovery.EnvironmentStatuses, databases []qovery.Database, blueprintDatabases []blueprintDatabase) string {
	var results []interface{}

	for _, database := range databases {
		host, port, credentials, err := nativeDatabaseConnection(&client, database)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		var login, password interface{}
		if credentials != nil {
			login, password = credentials.Login, credentials.Password
		}

		results = append(results, map[string]interface{}{
			"id":            database.Id,
			"updated_at":    utils.ToIso8601(database.UpdatedAt),
			"name":          database.Name,
			"type":          "Database",
			"database_type": database.Type,
			"status":        utils.FindStatus(statuses.GetDatabases(), database.Id),
			"host":          host,
			"port":          port,
			"login":         login,
			"password":      password,
		})
	}

	for _, database := range blueprintDatabases {
		// Same field types as the databases above; null when the blueprint has not reported them yet
		var host, login, password interface{}
		var port interface{}
		switch {
		case database.credentials != nil:
			host, port = database.credentials.Host, database.credentials.Port
			if showCredentials {
				login, password = database.credentials.Login, database.credentials.Password
			}
		case database.endpoint != nil:
			host = database.endpoint.Host
			if database.endpoint.Port.IsSet() && database.endpoint.Port.Get() != nil {
				port = *database.endpoint.Port.Get()
			}
		}

		results = append(results, map[string]interface{}{
			"id":            database.terraform.Id,
			"updated_at":    utils.ToIso8601(database.terraform.UpdatedAt),
			"name":          database.terraform.Name,
			"type":          "Blueprint database",
			"database_type": database.kind,
			"status":        utils.FindStatus(statuses.GetTerraforms(), database.terraform.Id),
			"host":          host,
			"port":          port,
			"login":         login,
			"password":      password,
		})
	}

	j, err := json.Marshal(results)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	return string(j)
}

// nativeDatabaseConnection fetches master credentials only when they are going to be shown; otherwise host and port
// come from the database itself and credentials is nil.
func nativeDatabaseConnection(client *qovery.APIClient, database qovery.Database) (string, int32, *qovery.Credentials, error) {
	if !showCredentials {
		return database.GetHost(), database.GetPort(), nil, nil
	}

	credentials, _, err := client.DatabaseMainCallsAPI.GetDatabaseMasterCredentials(context.Background(), database.Id).Execute()
	if err != nil {
		return "", 0, nil, err
	}

	return credentials.Host, credentials.Port, credentials, nil
}

// blueprintDatabase is a terraform service created by a database blueprint. credentials is nil until a deploy has
// reported them.
type blueprintDatabase struct {
	terraform   qovery.TerraformResponse
	kind        qovery.DatabaseTypeEnum
	endpoint    *qovery.BlueprintDatabaseResponseEndpoint
	credentials *qovery.Credentials
}

func (d blueprintDatabase) connection() (host string, port string, login string, password string) {
	switch {
	case d.credentials != nil:
		return d.credentials.Host, strconv.Itoa(int(d.credentials.Port)), d.credentials.Login, d.credentials.Password
	case d.endpoint != nil && d.endpoint.Port.IsSet() && d.endpoint.Port.Get() != nil:
		return d.endpoint.Host, strconv.Itoa(int(*d.endpoint.Port.Get())), "N/A", "N/A"
	case d.endpoint != nil:
		return d.endpoint.Host, "N/A", "N/A", "N/A"
	default:
		return "N/A", "N/A", "N/A", "N/A"
	}
}

// Master credentials are fetched only when they are going to be shown.
func listBlueprintDatabases(client *qovery.APIClient, envId string, withCredentials bool) ([]blueprintDatabase, error) {
	terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), envId).Execute()
	if err != nil {
		return nil, err
	}

	var databases []blueprintDatabase
	for _, terraform := range terraforms.GetResults() {
		blueprintId := terraform.GetBlueprintId()
		if blueprintId == "" {
			continue
		}

		database, res, err := client.BlueprintMainCallsAPI.GetBlueprintDatabase(context.Background(), blueprintId).Execute()
		if res != nil && res.StatusCode == http.StatusNotFound {
			// the blueprint is not a database
			continue
		}
		if err != nil {
			return nil, err
		}

		var credentials *qovery.Credentials
		if withCredentials {
			credentials, res, err = client.BlueprintMainCallsAPI.GetBlueprintDatabaseMasterCredentials(context.Background(), blueprintId).Execute()
			if res != nil && (res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden) {
				// not deployed yet, or the user may see the database but not its master credentials
				credentials = nil
			} else if err != nil {
				return nil, err
			}
		}

		databases = append(databases, blueprintDatabase{
			terraform:   terraform,
			kind:        database.Kind,
			endpoint:    database.Endpoint.Get(),
			credentials: credentials,
		})
	}

	return databases, nil
}

func init() {
	databaseCmd.AddCommand(databaseListCmd)
	databaseListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseListCmd.Flags().BoolVarP(&showCredentials, "show-credentials", "", false, "Show Credentials")
	databaseListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
