package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cronjobs",
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

		cronjobs, err := ListCronjobs(envId, client)

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
			fmt.Print(getCronjobJsonOutput(statuses.GetJobs(), cronjobs))
			return
		}

		var data [][]string

		for _, cronjob := range cronjobs {
			if cronjob.CronJobResponse != nil {
				data = append(data, []string{cronjob.CronJobResponse.Id, cronjob.CronJobResponse.Name, "Cronjob",
					utils.FindStatusTextWithColor(statuses.GetJobs(), cronjob.CronJobResponse.Id), cronjob.CronJobResponse.UpdatedAt.String()})
			}
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getCronjobJsonOutput(statuses []qovery.Status, cronjobs []qovery.JobResponse) string {
	var results []interface{}

	for _, cronjob := range cronjobs {
		if cronjob.CronJobResponse != nil {
			results = append(results, map[string]interface{}{
				"id":         cronjob.CronJobResponse.Id,
				"name":       cronjob.CronJobResponse.Name,
				"type":       "Cronjob",
				"status":     utils.FindStatus(statuses, cronjob.CronJobResponse.Id),
				"updated_at": utils.ToIso8601(cronjob.CronJobResponse.UpdatedAt),
			})
		}
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
	cronjobCmd.AddCommand(cronjobListCmd)
	cronjobListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
