package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	adminClusterOperatorJSON     bool
	adminClusterOperatorFilters  map[string]string
	adminClusterOperatorOutdated bool
)

var clusterOperatorFilterKeys = []string{
	"OrganizationId",
	"ClusterId",
	"ClusterName",
	"ClusterKind",
	"Runtime",
	"Status",
	"Connected",
}

var adminClusterOperatorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the Qovery Operator fleet",
	Long: "List the clusters attached to the Qovery Operator execution path.\n\n" +
		clusterOperatorFilterHelp() + `
Examples:
qovery admin cluster operator list --outdated
qovery admin cluster operator list -f Runtime=KUBERNETES -f Status=OUTDATED_IMAGE,OUTDATED_CHART
qovery admin cluster operator list -f Connected=false --json
qovery admin cluster operator list -f ClusterKind=EKS_ANYWHERE
qovery admin cluster operator list -f Runtime=DOCKER,KUBERNETES -f Connected=true`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if _, err := parseClusterOperatorFilters(adminClusterOperatorFilters); err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}
		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}
		fleet, err := getClusterOperatorFleet(
			context.Background(),
			utils.GetAdminUrl(),
			utils.GetAuthorizationHeaderValue(tokenType, token),
			&http.Client{Timeout: 60 * time.Second},
		)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}

		clusters, err := selectClusterOperators(fleet.GetResults(), adminClusterOperatorFilters, adminClusterOperatorOutdated)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}
		if adminClusterOperatorJSON {
			output, err := json.MarshalIndent(clusters, "", "  ")
			if err != nil {
				utils.PrintlnErrorToStderr(err)
				os.Exit(1)
			}
			utils.Println(string(output))
			return
		}

		organizationNames := lookupOrganizationNames(
			context.Background(),
			clusterOrganizationIDs(clusters),
			organizationNameLookup(tokenType, token),
			15*time.Second,
			10*time.Second,
		)
		if err := utils.PrintTable(
			[]string{
				"Organization",
				"Cluster ID",
				"Cluster",
				"Kind",
				"Runtime",
				"Attached",
				"Connected",
				"Last heartbeat",
				"Status",
				"Image",
				"Target image",
				"Chart",
				"Target chart",
			},
			clusterOperatorFleetRows(clusters, organizationNames),
		); err != nil {
			utils.PrintlnErrorToStderr(err)
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

var clusterOperatorFleetStatusDescriptions = map[qovery.ClusterOperatorFleetStatus]string{
	qovery.CLUSTEROPERATORFLEETSTATUS_NOT_ATTACHED:             "cluster not attached to the Operator path (never selected: only attached clusters are)",
	qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED:             "no Operator heartbeat in the last 2 minutes",
	qovery.CLUSTEROPERATORFLEETSTATUS_TARGET_UNKNOWN:           "q-core has no target image or chart version",
	qovery.CLUSTEROPERATORFLEETSTATUS_REPORTED_VERSION_UNKNOWN: "the Operator did not report its image or chart version",
	qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT:                  "reported image and chart match the target",
	qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE:           "reported image differs from the target",
	qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_CHART:           "reported chart differs from the target",
	qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE_AND_CHART: "reported image and chart both differ from the target",
}

func clusterOperatorFilterHelp() string {
	var help strings.Builder
	help.WriteString(`Filters (-f, --filters Key=Values):
  Comma separated values are a logical OR; different keys are a logical AND.
  Values are matched exactly and are case-sensitive, except Connected. A key repeated in several -f keeps only
  the last one: put all values of a key in one -f, separated by commas.

`)
	keys := [][2]string{
		{"OrganizationId", "organization UUID"},
		{"ClusterId", "cluster UUID"},
		{"ClusterName", "exact cluster name"},
		{"ClusterKind", strings.Join(enumStrings(qovery.AllowedSelfManagedClusterKindEnumValues), ", ")},
		{"Runtime", strings.Join(enumStrings(qovery.AllowedClusterOperatorRuntimeEnumValues), ", ")},
		{"Status", "a fleet status below"},
		{"Connected", "true or false"},
	}
	for _, key := range keys {
		fmt.Fprintf(&help, "  %-15s %s\n", key[0], key[1])
	}
	help.WriteString("\nFleet statuses:\n")
	for _, status := range qovery.AllowedClusterOperatorFleetStatusEnumValues {
		fmt.Fprintf(&help, "  %-25s %s\n", status, clusterOperatorFleetStatusDescriptions[status])
	}
	return help.String()
}

func selectClusterOperators(
	clusters []qovery.ClusterOperatorFleetInventoryResponse,
	filters map[string]string,
	outdatedOnly bool,
) ([]qovery.ClusterOperatorFleetInventoryResponse, error) {
	parsedFilters, err := parseClusterOperatorFilters(filters)
	if err != nil {
		return nil, err
	}
	selected := make([]qovery.ClusterOperatorFleetInventoryResponse, 0, len(clusters))
	for _, cluster := range attachedClusterOperators(clusters) {
		if outdatedOnly && !isClusterOperatorOutdated(cluster.Status) {
			continue
		}
		if matchesClusterOperatorFilters(cluster, parsedFilters) {
			selected = append(selected, cluster)
		}
	}
	return selected, nil
}

func parseClusterOperatorFilters(filters map[string]string) (map[string]map[string]bool, error) {
	parsed := make(map[string]map[string]bool, len(filters))
	for key, value := range filters {
		if !slices.Contains(clusterOperatorFilterKeys, key) {
			return nil, fmt.Errorf("filter property '%s' not available: valid values are: %s", key, strings.Join(clusterOperatorFilterKeys, ", "))
		}
		values := make(map[string]bool)
		for _, item := range strings.Split(value, ",") {
			normalized, err := normalizeClusterOperatorFilterValue(key, strings.TrimSpace(item))
			if err != nil {
				return nil, err
			}
			values[normalized] = true
		}
		parsed[key] = values
	}
	return parsed, nil
}

func normalizeClusterOperatorFilterValue(key string, value string) (string, error) {
	var allowed []string
	switch key {
	case "ClusterKind":
		allowed = enumStrings(qovery.AllowedSelfManagedClusterKindEnumValues)
	case "Runtime":
		allowed = enumStrings(qovery.AllowedClusterOperatorRuntimeEnumValues)
	case "Status":
		allowed = enumStrings(qovery.AllowedClusterOperatorFleetStatusEnumValues)
	case "Connected":
		connected, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("invalid value '%s' for filter Connected: valid values are: true, false", value)
		}
		return strconv.FormatBool(connected), nil
	default:
		return value, nil
	}
	if !slices.Contains(allowed, value) {
		return "", fmt.Errorf("invalid value '%s' for filter %s: valid values are: %s", value, key, strings.Join(allowed, ", "))
	}
	return value, nil
}

func enumStrings[T ~string](values []T) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func matchesClusterOperatorFilters(cluster qovery.ClusterOperatorFleetInventoryResponse, filters map[string]map[string]bool) bool {
	for key, values := range filters {
		if !values[clusterOperatorFilterValue(cluster, key)] {
			return false
		}
	}
	return true
}

func clusterOperatorFilterValue(cluster qovery.ClusterOperatorFleetInventoryResponse, key string) string {
	switch key {
	case "OrganizationId":
		return cluster.OrganizationId
	case "ClusterId":
		return cluster.ClusterId
	case "ClusterName":
		return cluster.ClusterName
	case "ClusterKind":
		return string(cluster.ClusterKind)
	case "Runtime":
		return string(cluster.ReportedRuntime)
	case "Status":
		return string(cluster.Status)
	case "Connected":
		return strconv.FormatBool(cluster.Connected)
	default:
		return ""
	}
}

func isClusterOperatorOutdated(status qovery.ClusterOperatorFleetStatus) bool {
	switch status {
	case qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE,
		qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_CHART,
		qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE_AND_CHART:
		return true
	default:
		return false
	}
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
		return nil, newHTTPStatusError("operator fleet API", response)
	}

	var fleet qovery.ClusterOperatorFleetInventoryResponseList
	if err := json.NewDecoder(response.Body).Decode(&fleet); err != nil {
		return nil, err
	}
	return &fleet, nil
}

func sortClusterOperators(clusters []qovery.ClusterOperatorFleetInventoryResponse) {
	sort.Slice(clusters, func(left int, right int) bool {
		if clusters[left].OrganizationId == clusters[right].OrganizationId {
			return clusters[left].ClusterName < clusters[right].ClusterName
		}
		return clusters[left].OrganizationId < clusters[right].OrganizationId
	})
}

func clusterOperatorFleetRows(
	clusters []qovery.ClusterOperatorFleetInventoryResponse,
	organizationNames map[string]string,
) [][]string {
	sortClusterOperators(clusters)

	rows := make([][]string, 0, len(clusters))
	for _, cluster := range clusters {
		lastHeartbeat := "never"
		if heartbeat := cluster.LastHeartbeat.Get(); heartbeat != nil {
			lastHeartbeat = heartbeat.Format(time.RFC3339)
		}
		reportedChart := displayVersion(cluster.ReportedChartVersion)
		desiredChart := displayVersion(cluster.DesiredChartVersion)
		if cluster.ReportedRuntime == qovery.CLUSTEROPERATORRUNTIME_DOCKER {
			reportedChart = "-"
			desiredChart = "-"
		}
		rows = append(rows, []string{
			organizationDisplayName(organizationNames, cluster.OrganizationId),
			cluster.ClusterId,
			cluster.ClusterName,
			string(cluster.ClusterKind),
			string(cluster.ReportedRuntime),
			strconv.FormatBool(cluster.Attached),
			strconv.FormatBool(cluster.Connected),
			lastHeartbeat,
			string(cluster.Status),
			displayVersion(cluster.ReportedImageVersion),
			displayVersion(cluster.DesiredImageVersion),
			reportedChart,
			desiredChart,
		})
	}
	return rows
}

func init() {
	adminClusterOperatorCmd.AddCommand(adminClusterOperatorListCmd)
	adminClusterOperatorListCmd.Flags().BoolVar(&adminClusterOperatorJSON, "json", false, "JSON output")
	adminClusterOperatorListCmd.Flags().StringToStringVarP(&adminClusterOperatorFilters, "filters", "f", make(map[string]string), "Value(s) to filter the property selected separated by comma when multiple values are defined")
	adminClusterOperatorListCmd.Flags().BoolVar(&adminClusterOperatorOutdated, "outdated", false, "Only show clusters whose Operator image or chart is outdated")
}

type httpStatusError struct {
	operation  string
	statusCode int
	status     string
	body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s returned %s: %s", e.operation, e.status, e.body)
}

func newHTTPStatusError(operation string, response *http.Response) *httpStatusError {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return &httpStatusError{
		operation:  operation,
		statusCode: response.StatusCode,
		status:     response.Status,
		body:       strings.TrimSpace(string(body)),
	}
}

func isUnauthorizedError(err error) bool {
	var apiError *httpStatusError
	return errors.As(err, &apiError) && apiError.statusCode == http.StatusUnauthorized
}
