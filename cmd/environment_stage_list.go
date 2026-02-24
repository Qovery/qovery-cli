package cmd

import (
	"context"
	"encoding/json"
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
		utils.CheckError(err)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		utils.CheckError(err)

		stages, _, err := client.DeploymentStageMainCallsAPI.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()
		utils.CheckError(err)

		if jsonFlag {
			utils.Println(getEnvironmentStageJsonOutput(*client, stages.GetResults()))
			return
		}

		// Collect all skipped services across all stages
		var skippedData [][]string
		for _, stage := range stages.GetResults() {
			for _, service := range stage.GetServices() {
				if service.GetIsSkipped() {
					skippedData = append(skippedData, []string{
						service.Id,
						service.GetServiceType(),
						utils.GetServiceNameByIdAndType(client, service.GetServiceId(), service.GetServiceType()),
						stage.GetName(),
					})
				}
			}
		}

		// Show skipped services section first
		if len(skippedData) > 0 {
			pterm.DefaultSection.WithBottomPadding(0).Println("Skipped services (excluded from environment-level deployments)")
			utils.Println("")
			err = utils.PrintTable([]string{"Id", "Type", "Name", "Stage"}, skippedData)
			utils.Println("")
			utils.CheckError(err)
		}

		// Show each stage with only non-skipped services
		for _, stage := range stages.GetResults() {
			pterm.DefaultSection.WithBottomPadding(0).Println("deployment stage " + strconv.Itoa(int(stage.GetDeploymentOrder()+1)) + ": \"" + stage.GetName() + "\"")
			utils.Println("Stage id: " + stage.GetId())
			if stage.GetDescription() != "" {
				utils.Println(stage.GetDescription())
			}

			utils.Println("")

			var data [][]string
			for _, service := range stage.GetServices() {
				if !service.GetIsSkipped() {
					data = append(data, []string{
						service.Id,
						service.GetServiceType(),
						utils.GetServiceNameByIdAndType(client, service.GetServiceId(), service.GetServiceType()),
					})
				}
			}

			if len(data) == 0 {
				if len(stage.GetServices()) == 0 {
					utils.Println("<no service>")
				} else {
					utils.Println("<all services skipped>")
				}
			} else {
				err = utils.PrintTable([]string{"Id", "Type", "Name"}, data)
				utils.CheckError(err)
			}

			utils.Println("")
		}
	},
}

func getEnvironmentStageJsonOutput(client qovery.APIClient, stages []qovery.DeploymentStageResponse) string {
	var skippedServices []interface{}
	var results []interface{}

	for idx, stage := range stages {
		var services []interface{}

		for _, service := range stage.Services {
			entry := map[string]interface{}{
				"id":         service.ServiceId,
				"type":       service.ServiceType,
				"name":       utils.GetServiceNameByIdAndType(&client, service.GetServiceId(), service.GetServiceType()),
				"is_skipped": service.GetIsSkipped(),
			}
			services = append(services, entry)

			if service.GetIsSkipped() {
				skippedServices = append(skippedServices, map[string]interface{}{
					"id":    service.ServiceId,
					"type":  service.ServiceType,
					"name":  utils.GetServiceNameByIdAndType(&client, service.GetServiceId(), service.GetServiceType()),
					"stage": stage.Name,
				})
			}
		}

		results = append(results, map[string]interface{}{
			"stage_order":       idx + 1,
			"stage_id":          stage.Id,
			"stage_name":        stage.Name,
			"stage_description": stage.Description,
			"services":          services,
		})
	}

	j, err := json.Marshal(map[string]interface{}{
		"skipped_services": skippedServices,
		"stages":           results,
	})
	utils.CheckError(err)

	return string(j)
}

func init() {
	environmentStageCmd.AddCommand(environmentStageListCmd)
	environmentStageListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
