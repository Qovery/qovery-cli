package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, projectId, err := getOrganizationProjectContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentsAPI.GetProjectEnvironmentsStatus(context.Background(), projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getEnvironmentJsonOutput(statuses.GetResults(), environments.GetResults()))
			return
		}

		var data [][]string

		for _, env := range environments.GetResults() {
			data = append(data, []string{env.Id, env.GetName(), *env.ClusterName, string(env.Mode),
				utils.GetEnvironmentStatusWithColor(statuses.GetResults(), env.Id), env.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Cluster", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getEnvironmentJsonOutput(statuses []qovery.EnvironmentStatus, environments []qovery.Environment) string {
	var results []interface{}

	for _, env := range environments {
		results = append(results, map[string]interface{}{
			"id":           env.Id,
			"created_at":   utils.ToIso8601(&env.CreatedAt),
			"updated_at":   utils.ToIso8601(env.UpdatedAt),
			"name":         env.GetName(),
			"cluster_name": *env.ClusterName,
			"cluster_id":   env.ClusterId,
			"type":         string(env.Mode),
			"status":       utils.GetEnvironmentStatus(statuses, env.Id),
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
	environmentCmd.AddCommand(environmentListCmd)
	environmentListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
