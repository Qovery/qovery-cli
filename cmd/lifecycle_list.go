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

var lifecycleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List lifecycle jobs",
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

		lifecycles, err := ListLifecycleJobs(envId, client)

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
			fmt.Print(getLifecycleJsonOutput(statuses.GetJobs(), lifecycles))
			return
		}

		var data [][]string

		for _, lifecycle := range lifecycles {
			if lifecycle.LifecycleJobResponse != nil {
				data = append(data, []string{lifecycle.LifecycleJobResponse.Id, lifecycle.LifecycleJobResponse.Name, "Lifecycle",
					utils.FindStatusTextWithColor(statuses.GetJobs(), lifecycle.LifecycleJobResponse.Id), lifecycle.LifecycleJobResponse.UpdatedAt.String()})
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

func getLifecycleJsonOutput(statuses []qovery.Status, lifecycles []qovery.JobResponse) string {
	var results []interface{}

	for _, lifecycle := range lifecycles {
		if lifecycle.LifecycleJobResponse != nil {
			results = append(results, map[string]interface{}{
				"id":         lifecycle.LifecycleJobResponse.Id,
				"name":       lifecycle.LifecycleJobResponse.Name,
				"type":       "Lifecycle",
				"status":     utils.FindStatus(statuses, lifecycle.LifecycleJobResponse.Id),
				"updated_at": utils.ToIso8601(lifecycle.LifecycleJobResponse.UpdatedAt),
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
	lifecycleCmd.AddCommand(lifecycleListCmd)
	lifecycleListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
