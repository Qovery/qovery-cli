package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var terraformListCmd = &cobra.Command{
	Use:   "list",
	Short: "List terraform services",
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

		terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), envId).Execute()

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
			fmt.Print(getTerraformJsonOutput(statuses.GetTerraforms(), terraforms.GetResults()))
			return
		}

		var data [][]string

		for _, terraform := range terraforms.GetResults() {
			data = append(data, []string{
				terraform.Id,
				terraform.Name,
				"Terraform",
				utils.FindStatusTextWithColor(statuses.GetTerraforms(), terraform.Id),
				terraform.UpdatedAt.String(),
			})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getTerraformJsonOutput(statuses []qovery.Status, terraforms []qovery.TerraformResponse) string {
	var results []interface{}

	for _, terraform := range terraforms {
		results = append(results, map[string]interface{}{
			"id":         terraform.Id,
			"name":       terraform.Name,
			"type":       "Terraform",
			"status":     utils.FindStatus(statuses, terraform.Id),
			"updated_at": utils.ToIso8601(terraform.UpdatedAt),
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
	terraformCmd.AddCommand(terraformListCmd)
	terraformListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
