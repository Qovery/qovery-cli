package cmd

import (
	"context"
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

		environments, _, err := client.EnvironmentsApi.ListEnvironment(context.Background(), projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentsApi.GetProjectEnvironmentsStatus(context.Background(), projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		for _, env := range environments.GetResults() {
			data = append(data, []string{env.GetName(), *env.ClusterName, string(env.Mode),
				utils.GetEnvironmentStatus(statuses.GetResults(), env.Id), env.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Cluster", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentListCmd)
	environmentListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
}
