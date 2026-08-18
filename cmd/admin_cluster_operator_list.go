package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var adminClusterOperatorJSON bool

var adminClusterOperatorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the Qovery Operator fleet",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		fleet, err := getClusterOperatorFleet(
			context.Background(),
			utils.GetAdminUrl(),
			utils.GetAuthorizationHeaderValue(tokenType, token),
			&http.Client{Timeout: 60 * time.Second},
		)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		clusters := attachedClusterOperators(fleet.GetResults())
		if adminClusterOperatorJSON {
			output, err := json.MarshalIndent(clusters, "", "  ")
			if err != nil {
				utils.PrintlnError(err)
				return
			}
			utils.Println(string(output))
			return
		}

		if err := utils.PrintTable(
			[]string{
				"Organization ID",
				"Cluster ID",
				"Cluster",
				"Kind",
				"Attached",
				"Connected",
				"Last heartbeat",
				"Status",
				"Image",
				"Target image",
				"Chart",
				"Target chart",
			},
			clusterOperatorFleetRows(clusters),
		); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func attachedClusterOperators(clusters []qovery.ClusterOperatorFleetInventoryResponse) []qovery.ClusterOperatorFleetInventoryResponse {
	attached := make([]qovery.ClusterOperatorFleetInventoryResponse, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.Attached {
			attached = append(attached, cluster)
		}
	}
	return attached
}

func getClusterOperatorFleet(
	ctx context.Context,
	adminURL string,
	authorization string,
	httpClient *http.Client,
) (*qovery.ClusterOperatorFleetInventoryResponseList, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(adminURL, "/")+"/operator/clusters",
		nil,
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return nil, fmt.Errorf("operator fleet API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var fleet qovery.ClusterOperatorFleetInventoryResponseList
	if err := json.NewDecoder(response.Body).Decode(&fleet); err != nil {
		return nil, err
	}
	return &fleet, nil
}

func clusterOperatorFleetRows(clusters []qovery.ClusterOperatorFleetInventoryResponse) [][]string {
	sort.Slice(clusters, func(left int, right int) bool {
		if clusters[left].OrganizationId == clusters[right].OrganizationId {
			return clusters[left].ClusterName < clusters[right].ClusterName
		}
		return clusters[left].OrganizationId < clusters[right].OrganizationId
	})

	rows := make([][]string, 0, len(clusters))
	for _, cluster := range clusters {
		lastHeartbeat := "never"
		if heartbeat := cluster.LastHeartbeat.Get(); heartbeat != nil {
			lastHeartbeat = heartbeat.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			cluster.OrganizationId,
			cluster.ClusterId,
			cluster.ClusterName,
			string(cluster.ClusterKind),
			strconv.FormatBool(cluster.Attached),
			strconv.FormatBool(cluster.Connected),
			lastHeartbeat,
			string(cluster.Status),
			displayVersion(cluster.ReportedImageVersion),
			displayVersion(cluster.DesiredImageVersion),
			displayVersion(cluster.ReportedChartVersion),
			displayVersion(cluster.DesiredChartVersion),
		})
	}
	return rows
}

func init() {
	adminClusterOperatorCmd.AddCommand(adminClusterOperatorListCmd)
	adminClusterOperatorListCmd.Flags().BoolVar(&adminClusterOperatorJSON, "json", false, "JSON output")
}
