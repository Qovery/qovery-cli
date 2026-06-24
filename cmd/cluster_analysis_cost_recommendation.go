package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

var clusterAnalysisCostRecommendationCmd = &cobra.Command{
	Use:   "cost-recommendation",
	Short: "Start a cluster cost recommendation analysis, optionally wait for completion, then print its report",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		request, err := newCostRecommendationRequest()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		runClusterAnalysis(request)
	},
}

func newCostRecommendationRequest() (*qovery.ClusterAnalysisRequest, error) {
	outputFormat, err := parseAnalysisOutput(clusterAnalysisOutputFormat)
	if err != nil {
		return nil, err
	}

	request := qovery.NewClusterAnalysisRequest(qovery.CLUSTERANALYSISKIND_COST_RECOMMENDATION, outputFormat)
	if clusterAnalysisPrometheusUrl != "" {
		request.SetPrometheusUrl(clusterAnalysisPrometheusUrl)
	}
	if len(clusterAnalysisCmdArgs) > 0 {
		request.SetCmdArgs(clusterAnalysisCmdArgs)
	}

	return request, nil
}

func init() {
	clusterAnalysisCmd.AddCommand(clusterAnalysisCostRecommendationCmd)

	addClusterAnalysisRunFlags(clusterAnalysisCostRecommendationCmd)
	clusterAnalysisCostRecommendationCmd.Flags().StringVar(&clusterAnalysisPrometheusUrl, "prometheus-url", "", "Optional Prometheus URL")
	clusterAnalysisCostRecommendationCmd.Flags().StringArrayVar(&clusterAnalysisCmdArgs, "cmd-arg", nil, "Optional allowlisted command argument. Repeat for each argument, e.g. --cmd-arg=--history_duration --cmd-arg=336")
}
