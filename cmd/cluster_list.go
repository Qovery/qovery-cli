package cmd

import (
	"context"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List clusters",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		orgId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		clusters, _, err := client.ClustersApi.ListOrganizationCluster(context.Background(), orgId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		for _, cluster := range clusters.GetResults() {
			data = append(data, []string{cluster.Id, cluster.Name, "cluster",
				utils.GetStatusTextWithColor(*cluster.Status), cluster.UpdatedAt.String()})
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
	clusterCmd.AddCommand(clusterListCmd)
	clusterListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
}
