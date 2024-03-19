package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var environmentServicesStatusesCmd = &cobra.Command{
	Use:   "statuses",
	Short: "Get environment services statuses",
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

		 // Get env and services statuses
		 statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			j, err := json.Marshal(statuses)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			utils.Println(string(j))
			return
		}

		if statuses.Environment == nil {
			utils.PrintlnError(fmt.Errorf("environment status not found for `%s`", envId))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string
		for _, status := range statuses.Applications{
			data = append(data, []string{
				"application",
				status.Id,
				string(status.GetState()),
			})
		}
		for _, status := range statuses.Containers{
			data = append(data, []string{
				"container",
				status.Id,
				string(status.GetState()),
			})
		}
		for _, status := range statuses.Helms{
			data = append(data, []string{
				"helm",
				status.Id,
				string(status.GetState()),
			})
		}
		for _, status := range statuses.Jobs{
			data = append(data, []string{
				"job",
				status.Id,
				string(status.GetState()),
			})
		}
		for _, status := range statuses.Databases{
			data = append(data, []string{
				"database",
				status.Id,
				string(status.GetState()),
			})
		}

		utils.Println(fmt.Sprintf("\nEnvironment status: %s \n", statuses.Environment.GetState()))
		err = utils.PrintTable([]string{
			"Type",
			"ID",
			"Status",
		}, data)

		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot print services statuses: %s", err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentServicesStatusesCmd)
	environmentServicesStatusesCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentServicesStatusesCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentServicesStatusesCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentServicesStatusesCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
