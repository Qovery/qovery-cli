package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
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

		applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		if jsonFlag {
			utils.Println(getAppJsonOutput(applications.GetResults(), statuses))
			return
		}

		for _, application := range applications.GetResults() {
			data = append(data, []string{application.Id, application.Name, "Application",
				utils.FindStatusTextWithColor(statuses.GetApplications(), application.Id), application.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getAppJsonOutput(applications []qovery.Application, statuses *qovery.EnvironmentStatuses) string {
	var results []interface{}

	for _, application := range applications {
		results = append(results, map[string]interface{}{
			"id":          application.Id,
			"name":        application.Name,
			"type":        "Application",
			"status":      utils.FindStatus(statuses.GetApplications(), application.Id),
			"last_update": application.UpdatedAt.String(),
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
	applicationCmd.AddCommand(applicationListCmd)
	applicationListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}

