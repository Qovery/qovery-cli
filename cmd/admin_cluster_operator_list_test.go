package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

func TestGetClusterOperatorFleetUsesAdminRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/operator/clusters" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"results":[{"organization_id":"org-1","cluster_id":"cluster-1","cluster_name":"customer-cluster","cluster_kind":"SELF_MANAGED","attached":true,"connected":true,"reported_runtime":"KUBERNETES","status":"CURRENT"}]}`)
	}))
	defer server.Close()

	fleet, err := getClusterOperatorFleet(context.Background(), server.URL, "Bearer token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(fleet.Results) != 1 || fleet.Results[0].ClusterName != "customer-cluster" {
		t.Fatalf("unexpected fleet: %#v", fleet.Results)
	}
}

func TestClusterOperatorFleetRows(t *testing.T) {
	heartbeat := time.Date(2026, time.August, 18, 12, 28, 46, 0, time.UTC)
	current := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-1",
		"cluster-1",
		"customer-cluster",
		qovery.SELFMANAGEDCLUSTERKIND_SELF_MANAGED,
		true,
		true,
		qovery.CLUSTEROPERATORRUNTIME_KUBERNETES,
		qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT,
	)
	current.SetLastHeartbeat(heartbeat)
	current.SetReportedImageVersion("v1.203.0")
	current.SetDesiredImageVersion("v1.203.0")
	current.SetReportedChartVersion("0.2.1")
	current.SetDesiredChartVersion("0.2.1")
	disconnected := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-1",
		"cluster-2",
		"another-cluster",
		qovery.SELFMANAGEDCLUSTERKIND_EKS_SELF_MANAGED,
		true,
		false,
		qovery.CLUSTEROPERATORRUNTIME_DOCKER,
		qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED,
	)

	powens := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-2",
		"cluster-3",
		"powens",
		qovery.SELFMANAGEDCLUSTERKIND_EKS_ANYWHERE,
		true,
		true,
		qovery.CLUSTEROPERATORRUNTIME_DOCKER,
		qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT,
	)
	powens.SetLastHeartbeat(heartbeat)
	powens.SetReportedImageVersion("v1.203.0")
	powens.SetDesiredImageVersion("v1.203.0")
	powens.SetDesiredChartVersion("0.2.3")

	rows := clusterOperatorFleetRows(
		[]qovery.ClusterOperatorFleetInventoryResponse{*powens, *current, *disconnected},
		map[string]string{"org-1": "Acme"},
	)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0][0] != "Acme" || rows[1][0] != "Acme" {
		t.Fatalf("expected organization name, got %q and %q", rows[0][0], rows[1][0])
	}
	expectedDocker := []string{"org-2", "cluster-3", "powens", "EKS_ANYWHERE", "DOCKER", "true", "true", "2026-08-18T12:28:46Z", "CURRENT", "v1.203.0", "v1.203.0", "-", "-"}
	if !slices.Equal(rows[2], expectedDocker) {
		t.Fatalf("unexpected DOCKER row with organization ID fallback:\n got %#v\nwant %#v", rows[2], expectedDocker)
	}
	if rows[0][2] != "another-cluster" || rows[0][4] != "DOCKER" || rows[0][7] != "never" || rows[0][8] != "DISCONNECTED" {
		t.Fatalf("unexpected disconnected row: %#v", rows[0])
	}
	if rows[1][4] != "KUBERNETES" || rows[1][7] != "2026-08-18T12:28:46Z" || rows[1][9] != "v1.203.0" || rows[1][11] != "0.2.1" {
		t.Fatalf("unexpected current row: %#v", rows[1])
	}
}

func TestAttachedClusterOperators(t *testing.T) {
	attached := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-1",
		"cluster-1",
		"attached-cluster",
		qovery.SELFMANAGEDCLUSTERKIND_SELF_MANAGED,
		true,
		true,
		qovery.CLUSTEROPERATORRUNTIME_KUBERNETES,
		qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT,
	)
	notAttached := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-1",
		"cluster-2",
		"local-cluster",
		qovery.SELFMANAGEDCLUSTERKIND_SELF_MANAGED,
		false,
		false,
		qovery.CLUSTEROPERATORRUNTIME_UNKNOWN,
		qovery.CLUSTEROPERATORFLEETSTATUS_NOT_ATTACHED,
	)

	clusters := attachedClusterOperators([]qovery.ClusterOperatorFleetInventoryResponse{*notAttached, *attached})

	if len(clusters) != 1 || clusters[0].ClusterId != "cluster-1" {
		t.Fatalf("unexpected attached clusters: %#v", clusters)
	}
}

func fleetEntry(
	clusterID string,
	runtime qovery.ClusterOperatorRuntime,
	connected bool,
	status qovery.ClusterOperatorFleetStatus,
) qovery.ClusterOperatorFleetInventoryResponse {
	return *qovery.NewClusterOperatorFleetInventoryResponse(
		"org-"+clusterID,
		clusterID,
		"name-"+clusterID,
		qovery.SELFMANAGEDCLUSTERKIND_SELF_MANAGED,
		true,
		connected,
		runtime,
		status,
	)
}

func clusterIDs(clusters []qovery.ClusterOperatorFleetInventoryResponse) []string {
	ids := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		ids = append(ids, cluster.ClusterId)
	}
	return ids
}

func TestSelectClusterOperators(t *testing.T) {
	eksAnywhere := fleetEntry("c4", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE_AND_CHART)
	eksAnywhere.ClusterKind = qovery.SELFMANAGEDCLUSTERKIND_EKS_ANYWHERE
	notAttached := fleetEntry("c6", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE)
	notAttached.Attached = false
	fleet := []qovery.ClusterOperatorFleetInventoryResponse{
		fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT),
		fleetEntry("c2", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE),
		fleetEntry("c3", qovery.CLUSTEROPERATORRUNTIME_UNKNOWN, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED),
		eksAnywhere,
		fleetEntry("c5", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_CHART),
		notAttached,
	}

	cases := []struct {
		name     string
		filters  map[string]string
		outdated bool
		expected []string
	}{
		{name: "no filter keeps attached clusters", filters: map[string]string{}, expected: []string{"c1", "c2", "c3", "c4", "c5"}},
		{name: "runtime", filters: map[string]string{"Runtime": "DOCKER"}, expected: []string{"c2"}},
		{name: "status values are a logical OR", filters: map[string]string{"Status": "CURRENT,OUTDATED_CHART"}, expected: []string{"c1", "c5"}},
		{name: "connected", filters: map[string]string{"Connected": "false"}, expected: []string{"c3"}},
		{name: "kind", filters: map[string]string{"ClusterKind": "EKS_ANYWHERE"}, expected: []string{"c4"}},
		{name: "organization and cluster name", filters: map[string]string{"OrganizationId": "org-c1,org-c2", "ClusterName": "name-c2"}, expected: []string{"c2"}},
		{name: "cluster id", filters: map[string]string{"ClusterId": "c5"}, expected: []string{"c5"}},
		{name: "outdated", filters: map[string]string{}, outdated: true, expected: []string{"c2", "c4", "c5"}},
		{name: "outdated and runtime", filters: map[string]string{"Runtime": "KUBERNETES"}, outdated: true, expected: []string{"c4", "c5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			selected, err := selectClusterOperators(fleet, testCase.filters, testCase.outdated)
			if err != nil {
				t.Fatal(err)
			}
			if got := clusterIDs(selected); !slices.Equal(got, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, got)
			}
		})
	}
}

func TestSelectClusterOperatorsNormalizesConnected(t *testing.T) {
	fleet := []qovery.ClusterOperatorFleetInventoryResponse{
		fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT),
		fleetEntry("c2", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED),
	}
	selected, err := selectClusterOperators(fleet, map[string]string{"Connected": "TRUE"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := clusterIDs(selected); !slices.Equal(got, []string{"c1"}) {
		t.Fatalf("expected [c1], got %v", got)
	}
}

func TestSelectClusterOperatorsRejectsInvalidFilterValues(t *testing.T) {
	cases := []struct {
		filters  map[string]string
		expected string
	}{
		{filters: map[string]string{"Runtime": "KUBERNETES,docker"}, expected: "invalid value 'docker' for filter Runtime: valid values are: KUBERNETES, DOCKER, UNKNOWN"},
		{filters: map[string]string{"Status": "OUTDATED"}, expected: "invalid value 'OUTDATED' for filter Status: valid values are: NOT_ATTACHED,"},
		{filters: map[string]string{"ClusterKind": "EKS"}, expected: "invalid value 'EKS' for filter ClusterKind: valid values are:"},
		{filters: map[string]string{"Connected": "yes"}, expected: "invalid value 'yes' for filter Connected: valid values are: true, false"},
	}
	for _, testCase := range cases {
		_, err := selectClusterOperators(nil, testCase.filters, false)
		if err == nil || !strings.HasPrefix(err.Error(), testCase.expected) {
			t.Fatalf("expected error starting with %q, got %v", testCase.expected, err)
		}
	}
}

func TestSelectClusterOperatorsRejectsUnknownFilter(t *testing.T) {
	_, err := selectClusterOperators(nil, map[string]string{"CurrentStatus": "DEPLOYED"}, false)
	if err == nil || !strings.Contains(err.Error(), "CurrentStatus") {
		t.Fatalf("expected unknown filter error, got %v", err)
	}
}

func TestClusterOperatorFilterHelpListsEveryAllowedValue(t *testing.T) {
	help := clusterOperatorFilterHelp()
	lines := strings.Split(help, "\n")
	valuesOf := func(key string) []string {
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[0] == key {
				return strings.Split(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), key)), ", ")
			}
		}
		return nil
	}

	if got, want := valuesOf("ClusterKind"), enumStrings(qovery.AllowedSelfManagedClusterKindEnumValues); !slices.Equal(got, want) {
		t.Errorf("ClusterKind help lists %v, want %v", got, want)
	}
	if got, want := valuesOf("Runtime"), enumStrings(qovery.AllowedClusterOperatorRuntimeEnumValues); !slices.Equal(got, want) {
		t.Errorf("Runtime help lists %v, want %v", got, want)
	}
	for _, key := range clusterOperatorFilterKeys {
		if valuesOf(key) == nil {
			t.Errorf("filter key %s is not documented", key)
		}
	}
	for _, status := range qovery.AllowedClusterOperatorFleetStatusEnumValues {
		if len(valuesOf(string(status))) == 0 || clusterOperatorFleetStatusDescriptions[status] == "" {
			t.Errorf("fleet status %s is not documented", status)
		}
	}
	for _, command := range []*cobra.Command{adminClusterOperatorListCmd, adminClusterOperatorUpdateCmd} {
		if !strings.Contains(command.Long, help) {
			t.Errorf("%s help does not include the filter help", command.CommandPath())
		}
	}
}
