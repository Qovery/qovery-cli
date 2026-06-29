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
	Example: `  qovery cluster analysis cost-recommendation \
    -c <cluster_id> \
    --output json \
    --cmd-arg=--history_duration \
    --cmd-arg=336 \
    --cmd-arg=--timeframe_duration \
    --cmd-arg=2.5 \
    --cmd-arg=--cpu-request \
    --cmd-arg=99 \
    --cmd-arg=--cpu-limit \
    --cmd-arg=99 \
    --cmd-arg=--memory-buffer-percentage \
    --cmd-arg=15 \
    --cmd-arg=--use-oomkill-data \
    --cmd-arg=--oom-memory-buffer-percentage \
    --cmd-arg=25 \
    --cmd-arg=--allow-hpa`,
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
