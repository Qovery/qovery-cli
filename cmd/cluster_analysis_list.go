package cmd

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

var clusterAnalysisListCmd = &cobra.Command{
	Use:   "list",
	Short: "List previous analyses for a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()

		analyses, res, err := client.ClustersAPI.ListClusterAnalyses(context.Background(), clusterAnalysisClusterId).Execute()
		if err != nil {
			utils.PrintlnError(httpError(res, err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if clusterAnalysisJson {
			utils.Println(getAnalysisJsonOutput(analyses.GetResults()))
			return
		}

		var data [][]string
		for _, a := range analyses.GetResults() {
			data = append(data, []string{
				a.GetId(),
				string(a.GetKind()),
				string(a.GetStatus()),
				a.GetCreatedAt().Format(time.RFC3339),
				a.GetTriggeredBy(),
				a.GetErrorMessage(),
			})
		}

		err = utils.PrintTable([]string{"Id", "Kind", "Status", "Created At", "Triggered By", "Error"}, data)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getAnalysisJsonOutput(analyses []qovery.ClusterAnalysisResponse) string {
	var results []interface{}
	for _, a := range analyses {
		createdAt := a.GetCreatedAt()
		updatedAt := a.GetUpdatedAt()
		results = append(results, map[string]interface{}{
			"id":           a.GetId(),
			"cluster_id":   a.GetClusterId(),
			"kind":         a.GetKind(),
			"status":       a.GetStatus(),
			"created_at":   utils.ToIso8601(&createdAt),
			"updated_at":   utils.ToIso8601(&updatedAt),
			"triggered_by": a.GetTriggeredBy(),
			"error":        a.GetErrorMessage(),
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
	clusterAnalysisCmd.AddCommand(clusterAnalysisListCmd)
	clusterAnalysisListCmd.Flags().StringVarP(&clusterAnalysisClusterId, "cluster-id", "c", "", "Cluster ID")
	clusterAnalysisListCmd.Flags().BoolVar(&clusterAnalysisJson, "json", false, "JSON output")
	_ = clusterAnalysisListCmd.MarkFlagRequired("cluster-id")
}
