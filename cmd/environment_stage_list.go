package cmd

import (
	"context"
	"github.com/pterm/pterm"
	"os"
	"strconv"

	"github.com/qovery/qovery-cli/utils"
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

		stages, _, err := client.DeploymentStageMainCallsApi.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		for _, stage := range stages.GetResults() {
			pterm.DefaultSection.WithBottomPadding(0).Println("deployment stage " + strconv.Itoa(int(stage.GetDeploymentOrder()+1)) + ": \"" + stage.GetName() + "\"")
			if stage.GetDescription() != "" {
				pterm.Println(stage.GetDescription())
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

func init() {
	environmentStageCmd.AddCommand(environmentStageListCmd)
	environmentStageListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
