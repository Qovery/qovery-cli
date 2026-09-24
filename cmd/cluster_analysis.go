package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

var (
	clusterAnalysisClusterId        string
	clusterAnalysisId               string
	clusterAnalysisOutputFormat     string
	clusterAnalysisPrometheusUrl    string
	clusterAnalysisCmdArgs          []string
	clusterAnalysisTargetK8sVersion string
	clusterAnalysisWatch            bool
	clusterAnalysisNoLogs           bool
	clusterAnalysisJson             bool
)

var clusterAnalysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "Run and inspect read-only cluster analyses (e.g. cost recommendations)",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	clusterCmd.AddCommand(clusterAnalysisCmd)
}

// parseAnalysisOutput maps a CLI --output value to the engine output format.
func parseAnalysisOutput(s string) (qovery.ClusterAnalysisOutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "json":
		return qovery.CLUSTERANALYSISOUTPUTFORMAT_JSON, nil
	case "table":
		return qovery.CLUSTERANALYSISOUTPUTFORMAT_TABLE, nil
	case "csv":
		return qovery.CLUSTERANALYSISOUTPUTFORMAT_CSV, nil
	default:
		return "", fmt.Errorf("invalid output format %q (allowed: table, json, csv)", s)
	}
}

// isFinalAnalysisStatus reports whether the analysis reached a terminal state.
func isFinalAnalysisStatus(status qovery.ClusterAnalysisStatus) bool {
	switch status {
	case qovery.CLUSTERANALYSISSTATUS_SUCCEEDED,
		qovery.CLUSTERANALYSISSTATUS_FAILED,
		qovery.CLUSTERANALYSISSTATUS_TERMINATED:
		return true
	default:
		return false
	}
}

// httpError formats an API error using the response body when available.
func httpError(res *http.Response, err error) error {
	if res == nil {
		return err
	}

	if res.Body == nil {
		if err != nil {
			return fmt.Errorf("status code: %s: %w", res.Status, err)
		}
		return fmt.Errorf("status code: %s", res.Status)
	}

	defer func() { _ = res.Body.Close() }()
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		if err != nil {
			return fmt.Errorf("status code: %s ; cannot read response body: %w ; original error: %w", res.Status, readErr, err)
		}
		return fmt.Errorf("status code: %s ; cannot read response body: %w", res.Status, readErr)
	}

	if err != nil {
		return fmt.Errorf("status code: %s ; body: %s ; original error: %w", res.Status, string(body), err)
	}
	return fmt.Errorf("status code: %s ; body: %s", res.Status, string(body))
}

// printAnalysisLogs fetches and prints the persisted report/log lines of an analysis.
func printAnalysisLogs(client *qovery.APIClient, clusterId string, analysisId string) error {
	logs, res, err := client.ClustersAPI.ListClusterAnalysisLogs(context.Background(), clusterId, analysisId).Execute()
	if err != nil {
		return httpError(res, err)
	}

	utils.Println(analysisReportFromLogs(logs.GetResults()))

	return nil
}

func analysisReportFromLogs(logs []qovery.ClusterAnalysisLogResponse) string {
	lines := make([]string, 0, len(logs))
	for _, line := range logs {
		lines = append(lines, line.GetMessage())
	}
	return strings.Join(lines, "\n")
}
