package cmd

import (
	"context"
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

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		for _, lifecycle := range lifecycles {
			data = append(data, []string{lifecycle.Id, lifecycle.Name, "Lifecycle",
				utils.GetStatus(statuses.GetJobs(), lifecycle.Id), lifecycle.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleListCmd)
	lifecycleListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
