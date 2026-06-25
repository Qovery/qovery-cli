package cmd

import (
	"context"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

func runClusterAnalysis(request *qovery.ClusterAnalysisRequest) {
	client := utils.GetQoveryClientPanicInCaseOfError()
	ctx := context.Background()

	analysis, res, err := client.ClustersAPI.
		StartClusterAnalysis(ctx, clusterAnalysisClusterId).
		ClusterAnalysisRequest(*request).
		Execute()
	if err != nil {
		utils.PrintlnError(httpError(res, err))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	analysisId := analysis.GetId()
	utils.Println("Analysis " + pterm.FgBlue.Sprintf("%s", analysisId) + " started (" + string(analysis.GetStatus()) + ")")

	if !clusterAnalysisWatch {
		utils.PrintlnInfo("Run 'qovery cluster analysis logs --cluster-id " + clusterAnalysisClusterId + " --analysis-id " + analysisId + "' to fetch the report once finished.")
		return
	}

	lastStatus := analysis.GetStatus()
	for !isFinalAnalysisStatus(lastStatus) {
		time.Sleep(5 * time.Second)

		current, res, err := client.ClustersAPI.GetClusterAnalysis(ctx, clusterAnalysisClusterId, analysisId).Execute()
		if err != nil {
			utils.PrintlnError(httpError(res, err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if current.GetStatus() != lastStatus {
			lastStatus = current.GetStatus()
			utils.Println("Status: " + string(lastStatus))
		}

		if isFinalAnalysisStatus(current.GetStatus()) {
			lastStatus = current.GetStatus()
			if errMsg := current.GetErrorMessage(); errMsg != "" {
				utils.Println(pterm.Error.Sprintf("%s", errMsg))
			}
			break
		}
	}

	if !clusterAnalysisNoLogs {
		if err := printAnalysisLogs(client, clusterAnalysisClusterId, analysisId); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	}

	if lastStatus != qovery.CLUSTERANALYSISSTATUS_SUCCEEDED {
		utils.Println(pterm.Error.Sprintf("Analysis %s ended with status %s", analysisId, string(lastStatus)))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	utils.Println(pterm.FgGreen.Sprintf("Analysis %s succeeded", analysisId))
}

func addClusterAnalysisRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&clusterAnalysisClusterId, "cluster-id", "c", "", "Cluster ID")
	cmd.Flags().StringVar(&clusterAnalysisOutputFormat, "output", "json", "Report output: table, json, csv")
	cmd.Flags().BoolVar(&clusterAnalysisWatch, "watch", true, "Wait for the analysis to finish and print its report")
	cmd.Flags().BoolVar(&clusterAnalysisNoLogs, "no-logs", false, "Do not print the report logs when finished")
	_ = cmd.MarkFlagRequired("cluster-id")
}
