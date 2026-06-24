package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

var clusterAnalysisDeprecatedApiCmd = &cobra.Command{
	Use:   "deprecated-api",
	Short: "Start a deprecated Kubernetes API analysis, optionally wait for completion, then print its report",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		request, err := newDeprecatedApiRequest()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		runClusterAnalysis(request)
	},
}

func newDeprecatedApiRequest() (*qovery.ClusterAnalysisRequest, error) {
	outputFormat, err := parseAnalysisOutput(clusterAnalysisOutputFormat)
	if err != nil {
		return nil, err
	}

	request := qovery.NewClusterAnalysisRequest(qovery.CLUSTERANALYSISKIND_DEPRECATED_API_CHECK, outputFormat)
	if clusterAnalysisTargetK8sVersion != "" {
		request.SetTargetKubernetesVersion(clusterAnalysisTargetK8sVersion)
	}

	return request, nil
}

func init() {
	clusterAnalysisCmd.AddCommand(clusterAnalysisDeprecatedApiCmd)

	addClusterAnalysisRunFlags(clusterAnalysisDeprecatedApiCmd)
	clusterAnalysisDeprecatedApiCmd.Flags().StringVar(&clusterAnalysisTargetK8sVersion, "target-kubernetes-version", "", "Optional target Kubernetes version")
}
