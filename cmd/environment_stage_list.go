package cmd

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentStageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployment stages",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		stages, _, err := client.DeploymentStageMainCallsAPI.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getEnvironmentStageJsonOutput(*client, stages.GetResults()))
			return
		}

		for _, stage := range stages.GetResults() {
			pterm.DefaultSection.WithBottomPadding(0).Println("deployment stage " + strconv.Itoa(int(stage.GetDeploymentOrder()+1)) + ": \"" + stage.GetName() + "\"")
			utils.Println("Stage id: " + stage.GetId())
			if stage.GetDescription() != "" {
				utils.Println(stage.GetDescription())
			}

			utils.Println("")

			var data [][]string
			for _, service := range stage.GetServices() {
				data = append(data, []string{
					service.Id,
					service.GetServiceType(),
					utils.GetServiceNameByIdAndType(client, service.GetServiceId(), service.GetServiceType()),
				})
			}

			if len(stage.GetServices()) == 0 {
				utils.Println("<no service>")
			} else {
				err = utils.PrintTable([]string{"Id", "Type", "Name"}, data)
			}

			utils.Println("")

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}
	},
}

func getEnvironmentStageJsonOutput(client qovery.APIClient, stages []qovery.DeploymentStageResponse) string {
	var results []interface{}

	for idx, stage := range stages {
		var services []interface{}

		for _, service := range stage.Services {
			services = append(services, map[string]interface{}{
				"id":   service.ServiceId,
				"type": service.ServiceType,
				"name": utils.GetServiceNameByIdAndType(&client, service.GetServiceId(), service.GetServiceType()),
			})
		}

		results = append(results, map[string]interface{}{
			"stage_order":       idx + 1,
			"stage_id":          stage.Id,
			"stage_name":        stage.Name,
			"stage_description": stage.Description,
			"services":          services,
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
	environmentStageCmd.AddCommand(environmentStageListCmd)
	environmentStageListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
