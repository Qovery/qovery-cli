package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var containerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List containers",
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

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

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
			utils.Println(getContainerJsonOutput(containers.GetResults(), statuses))
			return
		}

		var data [][]string

		for _, container := range containers.GetResults() {
			data = append(data, []string{container.Id, container.Name, "Container",
				utils.FindStatusTextWithColor(statuses.GetContainers(), container.Id), container.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getContainerJsonOutput(containers []qovery.ContainerResponse, statuses *qovery.EnvironmentStatuses) string {
	var results []interface{}

	for _, container := range containers {
		results = append(results, map[string]interface{}{
			"id":          container.Id,
			"name":        container.Name,
			"type":        "Container",
			"status":      utils.FindStatus(statuses.GetApplications(), container.Id),
			"last_update": container.UpdatedAt.String(),
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
	containerCmd.AddCommand(containerListCmd)
	containerListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
