package cmd

import (
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
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
		organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		clusters, err := cluster.NewClusterService(client, &promptuifactory.PromptUiFactoryImpl{}).ListClusters(organizationId)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getClusterJsonOutput(clusters.GetResults()))
			return
		}

		var data [][]string
		for _, cluster := range clusters.GetResults() {
			data = append(data, []string{cluster.Id, cluster.Name, "cluster",
				utils.GetClusterStatusTextWithColor(*cluster.Status), cluster.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getClusterJsonOutput(clusters []qovery.Cluster) string {
	var results []interface{}

	for _, cluster := range clusters {
		results = append(results, map[string]interface{}{
			"id":         cluster.Id,
			"updated_at": utils.ToIso8601(cluster.UpdatedAt),
			"type":       "cluster",
			"name":       cluster.Name,
			"status":     cluster.Status,
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
	clusterCmd.AddCommand(clusterListCmd)
	clusterListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	clusterListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
