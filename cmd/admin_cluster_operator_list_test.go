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

	rows := clusterOperatorFleetRows([]qovery.ClusterOperatorFleetInventoryResponse{*current, *disconnected})

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
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

func TestSelectClusterOperatorsRejectsUnknownFilter(t *testing.T) {
	_, err := selectClusterOperators(nil, map[string]string{"CurrentStatus": "DEPLOYED"}, false)
	if err == nil || !strings.Contains(err.Error(), "CurrentStatus") {
		t.Fatalf("expected unknown filter error, got %v", err)
	}
}
