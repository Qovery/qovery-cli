package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

var clusterAnalysisLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print the report/logs of a past cluster analysis",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()

		logs, res, err := client.ClustersAPI.ListClusterAnalysisLogs(context.Background(), clusterAnalysisClusterId, clusterAnalysisId).Execute()
		if err != nil {
			utils.PrintlnError(httpError(res, err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if clusterAnalysisJson {
			utils.Println(getAnalysisLogsJsonOutput(logs.GetResults()))
			return
		}

		utils.Println(analysisReportFromLogs(logs.GetResults()))
	},
}

func getAnalysisLogsJsonOutput(logs []qovery.ClusterAnalysisLogResponse) string {
	var results []interface{}
	for _, line := range logs {
		timestamp := line.GetTimestamp()
		results = append(results, map[string]interface{}{
			"timestamp":  utils.ToIso8601(&timestamp),
			"level":      line.GetLevel(),
			"message":    line.GetMessage(),
			"line_order": line.GetLineOrder(),
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
	clusterAnalysisCmd.AddCommand(clusterAnalysisLogsCmd)
	clusterAnalysisLogsCmd.Flags().StringVarP(&clusterAnalysisClusterId, "cluster-id", "c", "", "Cluster ID")
	clusterAnalysisLogsCmd.Flags().StringVarP(&clusterAnalysisId, "analysis-id", "a", "", "Analysis ID")
	clusterAnalysisLogsCmd.Flags().BoolVar(&clusterAnalysisJson, "json", false, "JSON output")
	_ = clusterAnalysisLogsCmd.MarkFlagRequired("cluster-id")
	_ = clusterAnalysisLogsCmd.MarkFlagRequired("analysis-id")
}
