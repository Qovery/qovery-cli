package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var environmentDeploymentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment deployments",
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

		deployments, _, err := client.EnvironmentDeploymentHistoryApi.ListEnvironmentDeploymentHistory(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		for _, deployment := range deployments.GetResults() {
			data = append(data, []string{
				deployment.Id,
				deployment.GetCreatedAt().String(),
				utils.GetStatusTextWithColor(deployment.GetStatus()),
				utils.GetDuration(deployment.GetCreatedAt(), deployment.GetUpdatedAt()),
			})
		}

		err = utils.PrintTable([]string{"Id", "Deployed At", "Status", "Duration"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	environmentDeploymentCmd.AddCommand(environmentDeploymentListCmd)
	environmentDeploymentListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeploymentListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeploymentListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
