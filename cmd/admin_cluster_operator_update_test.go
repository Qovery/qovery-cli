package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
)

func operatorCluster(
	runtime qovery.ClusterOperatorRuntime,
	connected bool,
	status qovery.ClusterOperatorFleetStatus,
	reportedImage string,
	reportedChart string,
	desiredChart string,
) qovery.ClusterOperatorFleetInventoryResponse {
	cluster := fleetEntry("cluster-1", runtime, connected, status)
	if reportedImage != "" {
		cluster.SetReportedImageVersion(reportedImage)
	}
	if reportedChart != "" {
		cluster.SetReportedChartVersion(reportedChart)
	}
	cluster.SetDesiredImageVersion("v2")
	if desiredChart != "" {
		cluster.SetDesiredChartVersion(desiredChart)
	}
	return cluster
}

func requestJSON(t *testing.T, request *qovery.ClusterOperatorUpdateRequest) string {
	t.Helper()
	if request == nil {
		return ""
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestClusterOperatorUpdateEligibility(t *testing.T) {
	kubernetes := qovery.CLUSTEROPERATORRUNTIME_KUBERNETES
	docker := qovery.CLUSTEROPERATORRUNTIME_DOCKER
	outdated := qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE
	current := qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT

	cases := []struct {
		name            string
		cluster         qovery.ClusterOperatorFleetInventoryResponse
		target          clusterOperatorUpdateTarget
		expectedRequest string
		expectedSkip    string
	}{
		{
			name:         "disconnected",
			cluster:      operatorCluster(kubernetes, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED, "v1", "0.1.0", "0.2.0"),
			expectedSkip: "operator disconnected",
		},
		{
			name:         "unknown runtime",
			cluster:      operatorCluster(qovery.CLUSTEROPERATORRUNTIME_UNKNOWN, true, outdated, "v1", "", ""),
			expectedSkip: "runtime UNKNOWN not supported",
		},
		{
			name:         "missing runtime",
			cluster:      operatorCluster("", true, outdated, "v1", "", ""),
			expectedSkip: "runtime UNKNOWN not supported",
		},
		{
			name:            "kubernetes outdated sends desired chart",
			cluster:         operatorCluster(kubernetes, true, outdated, "v1", "0.2.0", "0.2.0"),
			expectedRequest: `{"chart_version":"0.2.0"}`,
		},
		{
			name:         "kubernetes current",
			cluster:      operatorCluster(kubernetes, true, current, "v2", "0.2.0", "0.2.0"),
			expectedSkip: "status CURRENT",
		},
		{
			name:         "kubernetes without any chart target",
			cluster:      operatorCluster(kubernetes, true, outdated, "v1", "0.2.0", ""),
			expectedSkip: "no target chart version",
		},
		{
			name:            "docker outdated never sends chart",
			cluster:         operatorCluster(docker, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE_AND_CHART, "v1", "", "0.2.0"),
			expectedRequest: `{}`,
		},
		{
			name:         "docker current",
			cluster:      operatorCluster(docker, true, current, "v2", "", ""),
			expectedSkip: "status CURRENT",
		},
		{
			name:         "docker already on requested image",
			cluster:      operatorCluster(docker, true, current, "v3", "", ""),
			target:       clusterOperatorUpdateTarget{imageVersion: "v3"},
			expectedSkip: "already on requested version",
		},
		{
			name:            "docker requested image ignores requested chart",
			cluster:         operatorCluster(docker, true, current, "v2", "", ""),
			target:          clusterOperatorUpdateTarget{imageVersion: "v3", chartVersion: "0.3.0"},
			expectedRequest: `{"image_version":"v3"}`,
		},
		{
			name:         "docker with chart version only",
			cluster:      operatorCluster(docker, true, outdated, "v1", "", ""),
			target:       clusterOperatorUpdateTarget{chartVersion: "0.3.0"},
			expectedSkip: "chart version does not apply to DOCKER",
		},
		{
			name:            "kubernetes requested chart differs",
			cluster:         operatorCluster(kubernetes, true, current, "v2", "0.2.0", "0.2.0"),
			target:          clusterOperatorUpdateTarget{chartVersion: "0.3.0"},
			expectedRequest: `{"chart_version":"0.3.0"}`,
		},
		{
			name:         "kubernetes already on requested image and chart",
			cluster:      operatorCluster(kubernetes, true, outdated, "v3", "0.3.0", "0.2.0"),
			target:       clusterOperatorUpdateTarget{imageVersion: "v3", chartVersion: "0.3.0"},
			expectedSkip: "already on requested version",
		},
		{
			name:            "kubernetes requested image sends desired chart",
			cluster:         operatorCluster(kubernetes, true, current, "v2", "0.2.0", "0.2.0"),
			target:          clusterOperatorUpdateTarget{imageVersion: "v3"},
			expectedRequest: `{"chart_version":"0.2.0","image_version":"v3"}`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request, skipReason := clusterOperatorUpdateEligibility(testCase.cluster, testCase.target)
			if skipReason != testCase.expectedSkip {
				t.Fatalf("expected skip reason %q, got %q", testCase.expectedSkip, skipReason)
			}
			if got := requestJSON(t, request); got != testCase.expectedRequest {
				t.Fatalf("expected request %s, got %s", testCase.expectedRequest, got)
			}
		})
	}
}

func TestPlanClusterOperatorUpdatesKeepsSkippedClusters(t *testing.T) {
	updatable := operatorCluster(qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE, "v1", "", "")
	disconnected := fleetEntry("cluster-2", qovery.CLUSTEROPERATORRUNTIME_DOCKER, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED)

	plan := planClusterOperatorUpdates([]qovery.ClusterOperatorFleetInventoryResponse{updatable, disconnected}, clusterOperatorUpdateTarget{})
	rows := clusterOperatorUpdatePlanRows(plan, "", "")

	if len(eligibleClusterOperatorUpdates(plan)) != 1 {
		t.Fatalf("expected one eligible update, got %#v", plan)
	}
	if rows[0][5] != "v1 -> v2" || rows[0][6] != "-" || rows[0][7] != "update" {
		t.Fatalf("unexpected update row: %#v", rows[0])
	}
	if rows[1][7] != "skip: operator disconnected" {
		t.Fatalf("unexpected skipped row: %#v", rows[1])
	}
}

func TestPostClusterOperatorUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/cluster/operator/update/cluster-1" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing authorization header")
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != `{"image_version":"v3"}` {
			t.Errorf("unexpected body %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(writer, `{"execution_id":"exec-1","image_version":"v3","chart_version":null}`)
	}))
	defer server.Close()

	docker := operatorCluster(qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT, "v2", "", "0.2.0")
	request := buildClusterOperatorUpdateRequest(docker, clusterOperatorUpdateTarget{imageVersion: "v3", chartVersion: "0.3.0"})
	response, err := postClusterOperatorUpdate(context.Background(), server.URL+"/", "Bearer token", server.Client(), "cluster-1", *request)
	if err != nil {
		t.Fatal(err)
	}
	if response.ExecutionId != "exec-1" || response.ImageVersion != "v3" || response.ChartVersion.Get() != nil {
		t.Fatalf("unexpected response %#v", response)
	}
}

func TestPostClusterOperatorUpdateSurfacesErrors(t *testing.T) {
	statusCode := http.StatusPreconditionFailed
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(statusCode)
		_, _ = fmt.Fprint(writer, "operator is disconnected")
	}))
	defer server.Close()

	_, err := postClusterOperatorUpdate(context.Background(), server.URL, "Bearer token", server.Client(), "cluster-1", *qovery.NewClusterOperatorUpdateRequest())
	if err == nil || !strings.Contains(err.Error(), "412") || !strings.Contains(err.Error(), "operator is disconnected") {
		t.Fatalf("expected 412 error with body, got %v", err)
	}
	if isUnauthorizedError(err) {
		t.Fatal("412 must not be treated as unauthorized")
	}

	statusCode = http.StatusUnauthorized
	_, err = postClusterOperatorUpdate(context.Background(), server.URL, "Bearer token", server.Client(), "cluster-1", *qovery.NewClusterOperatorUpdateRequest())
	if !isUnauthorizedError(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestWithAdminReauthenticationRetriesOnceAfterUnauthorized(t *testing.T) {
	reauthentications := 0
	credentials := &adminCredentials{
		tokenType: "Bearer",
		token:     "expired",
		reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
			reauthentications++
			return "Bearer", "fresh", nil
		},
	}
	var tokens []utils.AccessToken
	result, err := withAdminReauthentication(credentials, func(_ utils.AccessTokenType, token utils.AccessToken) (string, error) {
		tokens = append(tokens, token)
		if token == "expired" {
			return "", &httpStatusError{statusCode: http.StatusUnauthorized, status: "401 Unauthorized"}
		}
		return "ok", nil
	})

	if err != nil || result != "ok" || reauthentications != 1 || len(tokens) != 2 || tokens[1] != "fresh" {
		t.Fatalf("unexpected retry: result=%q err=%v reauthentications=%d tokens=%v", result, err, reauthentications, tokens)
	}
}

func historyEntry(executionID string, status qovery.StateEnum) qovery.ClusterDeploymentHistory {
	entry := qovery.ClusterDeploymentHistory{Status: status, Operation: "OPERATOR_UPDATE"}
	entry.Identifier.SetExecutionId(executionID)
	return entry
}

func TestEvaluateClusterOperatorUpdateExecution(t *testing.T) {
	cases := []struct {
		name             string
		history          []qovery.ClusterDeploymentHistory
		expectedFinished bool
		expectedError    bool
	}{
		{name: "not recorded yet", history: nil},
		{name: "other execution failed", history: []qovery.ClusterDeploymentHistory{historyEntry("exec-0", qovery.STATEENUM_DEPLOYMENT_ERROR)}},
		{name: "running", history: []qovery.ClusterDeploymentHistory{historyEntry("exec-1", qovery.STATEENUM_DEPLOYING)}},
		{name: "deployed", history: []qovery.ClusterDeploymentHistory{historyEntry("exec-1", qovery.STATEENUM_DEPLOYED)}, expectedFinished: true},
		{name: "deployment error", history: []qovery.ClusterDeploymentHistory{historyEntry("exec-1", qovery.STATEENUM_DEPLOYMENT_ERROR)}, expectedFinished: true, expectedError: true},
		{name: "canceled", history: []qovery.ClusterDeploymentHistory{historyEntry("exec-1", qovery.STATEENUM_CANCELED)}, expectedFinished: true, expectedError: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			finished, err := evaluateClusterOperatorUpdateExecution(testCase.history, "exec-1")
			if finished != testCase.expectedFinished || (err != nil) != testCase.expectedError {
				t.Fatalf("expected finished=%t error=%t, got finished=%t err=%v", testCase.expectedFinished, testCase.expectedError, finished, err)
			}
		})
	}
}

func TestClusterOperatorReportsTarget(t *testing.T) {
	kubernetesResponse := qovery.NewClusterOperatorUpdateResponse("exec-1", "v3")
	kubernetesResponse.SetChartVersion("0.3.0")
	dockerResponse := qovery.NewClusterOperatorUpdateResponse("exec-1", "v3")
	dockerResponse.SetChartVersionNil()

	cases := []struct {
		name     string
		cluster  qovery.ClusterOperatorFleetInventoryResponse
		response *qovery.ClusterOperatorUpdateResponse
		expected bool
	}{
		{name: "kubernetes on target", cluster: operatorCluster(qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT, "v3", "0.3.0", ""), response: kubernetesResponse, expected: true},
		{name: "kubernetes old chart", cluster: operatorCluster(qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT, "v3", "0.2.0", ""), response: kubernetesResponse},
		{name: "old image", cluster: operatorCluster(qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT, "v2", "0.3.0", ""), response: kubernetesResponse},
		{name: "disconnected", cluster: operatorCluster(qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED, "v3", "0.3.0", ""), response: kubernetesResponse},
		{name: "docker on target", cluster: operatorCluster(qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT, "v3", "", ""), response: dockerResponse, expected: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fleet := []qovery.ClusterOperatorFleetInventoryResponse{testCase.cluster}
			if got := clusterOperatorReportsTarget(fleet, "cluster-1", *testCase.response); got != testCase.expected {
				t.Fatalf("expected %t, got %t", testCase.expected, got)
			}
		})
	}
	if clusterOperatorReportsTarget(nil, "cluster-1", *dockerResponse) {
		t.Fatal("a cluster missing from the fleet must not report the target")
	}
}

type fakeClusterOperatorUpdateAPI struct {
	mutex          sync.Mutex
	updateErrors   map[string]error
	historyStatus  qovery.StateEnum
	reportedImages map[string]string
	updated        []string
}

func (api *fakeClusterOperatorUpdateAPI) update(_ context.Context, clusterID string, _ qovery.ClusterOperatorUpdateRequest) (*qovery.ClusterOperatorUpdateResponse, error) {
	api.mutex.Lock()
	defer api.mutex.Unlock()
	api.updated = append(api.updated, clusterID)
	if err := api.updateErrors[clusterID]; err != nil {
		return nil, err
	}
	response := qovery.NewClusterOperatorUpdateResponse("exec-"+clusterID, "v3")
	response.SetChartVersionNil()
	return response, nil
}

func (api *fakeClusterOperatorUpdateAPI) fleet(context.Context) ([]qovery.ClusterOperatorFleetInventoryResponse, error) {
	api.mutex.Lock()
	defer api.mutex.Unlock()
	fleet := make([]qovery.ClusterOperatorFleetInventoryResponse, 0, len(api.reportedImages))
	for clusterID, image := range api.reportedImages {
		cluster := fleetEntry(clusterID, qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT)
		cluster.SetReportedImageVersion(image)
		fleet = append(fleet, cluster)
	}
	return fleet, nil
}

func (api *fakeClusterOperatorUpdateAPI) history(_ context.Context, _ string, clusterID string) ([]qovery.ClusterDeploymentHistory, error) {
	return []qovery.ClusterDeploymentHistory{historyEntry("exec-"+clusterID, api.historyStatus)}, nil
}

func dockerUpdates(clusterIDs ...string) []clusterOperatorUpdatePlanItem {
	updates := make([]clusterOperatorUpdatePlanItem, 0, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		updates = append(updates, clusterOperatorUpdatePlanItem{
			cluster: fleetEntry(clusterID, qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE),
			request: qovery.NewClusterOperatorUpdateRequest(),
		})
	}
	return updates
}

func testUpdateOptions(timeout time.Duration) clusterOperatorUpdateOptions {
	return clusterOperatorUpdateOptions{parallelRuns: 1, refreshDelay: time.Millisecond, timeout: timeout, print: func(string) {}}
}

func TestRunClusterOperatorUpdatesSucceedsWhenTargetIsReported(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYED, reportedImages: map[string]string{"c1": "v3", "c2": "v3"}}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1", "c2"), testUpdateOptions(time.Second))

	for _, result := range results {
		if !result.started || result.err != nil || result.executionID != "exec-"+result.cluster.ClusterId {
			t.Fatalf("unexpected result %#v", result)
		}
	}
}

func TestRunClusterOperatorUpdatesFailsOnHistoryError(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYMENT_ERROR, reportedImages: map[string]string{"c1": "v3"}}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1"), testUpdateOptions(time.Second))

	if results[0].err == nil || !strings.Contains(results[0].err.Error(), "DEPLOYMENT_ERROR") {
		t.Fatalf("expected history failure, got %#v", results[0])
	}
}

func TestRunClusterOperatorUpdatesTimesOutUntilTargetIsReported(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYED, reportedImages: map[string]string{"c1": "v2"}}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1"), testUpdateOptions(50*time.Millisecond))

	if results[0].err == nil || !strings.Contains(results[0].err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %#v", results[0])
	}
}

func TestRunClusterOperatorUpdatesStopsAfterFirstFailure(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{
		updateErrors:   map[string]error{"c2": errors.New("412 Precondition Failed")},
		historyStatus:  qovery.STATEENUM_DEPLOYED,
		reportedImages: map[string]string{"c1": "v3", "c2": "v3", "c3": "v3", "c4": "v3"},
	}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1", "c2", "c3", "c4"), testUpdateOptions(time.Second))

	if results[0].err != nil || !results[0].started {
		t.Fatalf("first cluster should succeed: %#v", results[0])
	}
	if results[1].err == nil || !strings.Contains(results[1].err.Error(), "412") {
		t.Fatalf("second cluster should fail: %#v", results[1])
	}
	if results[2].started || results[3].started {
		t.Fatalf("no update should start after a failure: %#v", results[2:])
	}
	if strings.Join(api.updated, ",") != "c1,c2" {
		t.Fatalf("unexpected update calls %v", api.updated)
	}
}

func TestNewClusterOperatorUpdateOptionsBounds(t *testing.T) {
	cases := []struct {
		parallelRuns int
		refreshDelay int
		timeout      time.Duration
		valid        bool
	}{
		{parallelRuns: 1, refreshDelay: 15, timeout: time.Minute, valid: true},
		{parallelRuns: 10, refreshDelay: 120, timeout: time.Minute, valid: true},
		{parallelRuns: 0, refreshDelay: 15, timeout: time.Minute},
		{parallelRuns: 11, refreshDelay: 15, timeout: time.Minute},
		{parallelRuns: 1, refreshDelay: 4, timeout: time.Minute},
		{parallelRuns: 1, refreshDelay: 121, timeout: time.Minute},
		{parallelRuns: 1, refreshDelay: 15, timeout: 0},
	}
	for _, testCase := range cases {
		_, err := newClusterOperatorUpdateOptions(testCase.parallelRuns, testCase.refreshDelay, testCase.timeout)
		if (err == nil) != testCase.valid {
			t.Fatalf("unexpected validation for %#v: %v", testCase, err)
		}
	}
}
