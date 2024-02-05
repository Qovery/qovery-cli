package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var helmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List helms",
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

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

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

		if jsonFlag {
			utils.Println(getHelmJsonOutput(helms.GetResults(), statuses))
			return
		}

		var data [][]string

		for _, helm := range helms.GetResults() {
			data = append(data, []string{helm.Id, helm.Name, "Helm",
				utils.FindStatusTextWithColor(statuses.GetHelms(), helm.Id), helm.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getHelmJsonOutput(helms []qovery.HelmResponse, statuses *qovery.EnvironmentStatuses) string {
	var results []interface{}

	for _, helm := range helms {
		results = append(results, map[string]interface{}{
			"id":          helm.Id,
			"name":        helm.Name,
			"type":        "Helm",
			"status":      utils.FindStatus(statuses.GetHelms(), helm.Id),
			"last_update": helm.UpdatedAt.String(),
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
	helmCmd.AddCommand(helmListCmd)
	helmListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
