package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		fmt.Fprint(writer, `{"results":[{"organization_id":"org-1","cluster_id":"cluster-1","cluster_name":"customer-cluster","cluster_kind":"SELF_MANAGED","attached":true,"connected":true,"status":"CURRENT"}]}`)
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
		qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED,
	)

	rows := clusterOperatorFleetRows([]qovery.ClusterOperatorFleetInventoryResponse{*current, *disconnected})

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][2] != "another-cluster" || rows[0][6] != "never" || rows[0][7] != "DISCONNECTED" {
		t.Fatalf("unexpected disconnected row: %#v", rows[0])
	}
	if rows[1][6] != "2026-08-18T12:28:46Z" || rows[1][8] != "v1.203.0" || rows[1][10] != "0.2.1" {
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
		qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT,
	)
	notAttached := qovery.NewClusterOperatorFleetInventoryResponse(
		"org-1",
		"cluster-2",
		"local-cluster",
		qovery.SELFMANAGEDCLUSTERKIND_SELF_MANAGED,
		false,
		false,
		qovery.CLUSTEROPERATORFLEETSTATUS_NOT_ATTACHED,
	)

	clusters := attachedClusterOperators([]qovery.ClusterOperatorFleetInventoryResponse{*notAttached, *attached})

	if len(clusters) != 1 || clusters[0].ClusterId != "cluster-1" {
		t.Fatalf("unexpected attached clusters: %#v", clusters)
	}
}
