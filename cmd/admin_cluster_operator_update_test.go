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
	"github.com/spf13/cobra"
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
			cluster:      operatorCluster(docker, true, current, "v1.3.0", "", ""),
			target:       clusterOperatorUpdateTarget{imageVersion: "v1.3.0"},
			expectedSkip: "already on requested version",
		},
		{
			name:            "docker requested image ignores requested chart",
			cluster:         operatorCluster(docker, true, current, "v2", "", ""),
			target:          clusterOperatorUpdateTarget{imageVersion: "v1.3.0", chartVersion: "0.3.0"},
			expectedRequest: `{"image_version":"v1.3.0"}`,
		},
		{
			name:            "docker requested pre-release tag",
			cluster:         operatorCluster(docker, true, current, "v2", "", ""),
			target:          clusterOperatorUpdateTarget{imageVersion: "v1.3.0-rc.1"},
			expectedRequest: `{"image_version":"v1.3.0-rc.1"}`,
		},
		{
			name:            "docker requested non-release tag equal to desired",
			cluster:         operatorCluster(docker, true, outdated, "v1", "", ""),
			target:          clusterOperatorUpdateTarget{imageVersion: "v2"},
			expectedRequest: `{"image_version":"v2"}`,
		},
		{
			name:         "docker requested non-release tag",
			cluster:      operatorCluster(docker, true, current, "v2", "", ""),
			target:       clusterOperatorUpdateTarget{imageVersion: "main-3f2a1c"},
			expectedSkip: "DOCKER heartbeat cannot confirm a non-release image tag",
		},
		{
			name:         "docker requested tag with build metadata",
			cluster:      operatorCluster(docker, true, current, "v2", "", ""),
			target:       clusterOperatorUpdateTarget{imageVersion: "v1.3.0+build"},
			expectedSkip: "DOCKER heartbeat cannot confirm a non-release image tag",
		},
		{
			name:         "docker requested tag with prefix",
			cluster:      operatorCluster(docker, true, current, "v2", "", ""),
			target:       clusterOperatorUpdateTarget{imageVersion: "release-v1.3.0"},
			expectedSkip: "DOCKER heartbeat cannot confirm a non-release image tag",
		},
		{
			name:            "kubernetes requested image already reported with stale chart",
			cluster:         operatorCluster(kubernetes, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_CHART, "v1.3.0", "0.1.0", "0.2.0"),
			target:          clusterOperatorUpdateTarget{imageVersion: "v1.3.0"},
			expectedRequest: `{"chart_version":"0.2.0","image_version":"v1.3.0"}`,
		},
		{
			name:         "kubernetes requested image already reported with desired chart",
			cluster:      operatorCluster(kubernetes, true, current, "v1.3.0", "0.2.0", "0.2.0"),
			target:       clusterOperatorUpdateTarget{imageVersion: "v1.3.0"},
			expectedSkip: "already on requested version",
		},
		{
			name:            "kubernetes requested chart already reported with outdated image",
			cluster:         operatorCluster(kubernetes, true, outdated, "v1", "0.3.0", "0.2.0"),
			target:          clusterOperatorUpdateTarget{chartVersion: "0.3.0"},
			expectedRequest: `{"chart_version":"0.3.0"}`,
		},
		{
			name:         "kubernetes requested chart with image and chart already current",
			cluster:      operatorCluster(kubernetes, true, current, "v2", "0.3.0", "0.2.0"),
			target:       clusterOperatorUpdateTarget{chartVersion: "0.3.0"},
			expectedSkip: "already on requested version",
		},
		{
			name:            "kubernetes requested non-release tag",
			cluster:         operatorCluster(kubernetes, true, current, "v2", "0.2.0", "0.2.0"),
			target:          clusterOperatorUpdateTarget{imageVersion: "main-3f2a1c"},
			expectedRequest: `{"chart_version":"0.2.0","image_version":"main-3f2a1c"}`,
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
	rows := clusterOperatorUpdatePlanRows(plan, "", "", map[string]string{"org-cluster-1": "Acme"})

	if len(eligibleClusterOperatorUpdates(plan)) != 1 {
		t.Fatalf("expected one eligible update, got %#v", plan)
	}
	if rows[0][0] != "Acme" || rows[1][0] != "org-cluster-2" {
		t.Fatalf("expected organization name with ID fallback, got %q and %q", rows[0][0], rows[1][0])
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
		tokenType:               "Bearer",
		token:                   "expired",
		browserReauthentication: true,
		reload: func() (utils.AccessTokenType, utils.AccessToken, error) {
			return "Bearer", "expired", nil
		},
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

func unauthorizedCall(_ utils.AccessTokenType, _ utils.AccessToken) (string, error) {
	return "", &httpStatusError{statusCode: http.StatusUnauthorized, status: "401 Unauthorized"}
}

func TestWithAdminReauthenticationPrefersSilentToken(t *testing.T) {
	reauthentications := 0
	credentials := &adminCredentials{
		tokenType:               "Bearer",
		token:                   "expired",
		browserReauthentication: true,
		reload: func() (utils.AccessTokenType, utils.AccessToken, error) {
			return "Bearer", "refreshed", nil
		},
		reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
			reauthentications++
			return "Bearer", "fresh", nil
		},
	}
	result, err := withAdminReauthentication(credentials, func(_ utils.AccessTokenType, token utils.AccessToken) (string, error) {
		if token != "refreshed" {
			return unauthorizedCall("", token)
		}
		return "ok", nil
	})

	if err != nil || result != "ok" || reauthentications != 0 {
		t.Fatalf("expected silent refresh: result=%q err=%v reauthentications=%d", result, err, reauthentications)
	}
}

func TestWithAdminReauthenticationOpensBrowserAtMostOnce(t *testing.T) {
	reauthentications := 0
	credentials := &adminCredentials{
		tokenType:               "Bearer",
		token:                   "expired",
		browserReauthentication: true,
		reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
			reauthentications++
			return "Bearer", "fresh", nil
		},
	}
	credentials.reload = func() (utils.AccessTokenType, utils.AccessToken, error) {
		return credentials.tokenType, credentials.token, nil
	}

	for attempt := 0; attempt < 3; attempt++ {
		if _, err := withAdminReauthentication(credentials, unauthorizedCall); !isUnauthorizedError(err) {
			t.Fatalf("attempt %d: expected unauthorized error, got %v", attempt, err)
		}
	}
	if reauthentications != 1 {
		t.Fatalf("expected one browser re-authentication, got %d", reauthentications)
	}
}

func TestWithAdminReauthenticationNeverOpensBrowserForEnvironmentToken(t *testing.T) {
	reauthentications := 0
	credentials := &adminCredentials{
		tokenType:               "Bearer",
		token:                   "environment",
		browserReauthentication: false,
		reload: func() (utils.AccessTokenType, utils.AccessToken, error) {
			return "Bearer", "environment", nil
		},
		reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
			reauthentications++
			return "Bearer", "fresh", nil
		},
	}

	if _, err := withAdminReauthentication(credentials, unauthorizedCall); !isUnauthorizedError(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
	if reauthentications != 0 {
		t.Fatalf("expected no browser re-authentication, got %d", reauthentications)
	}
}

func TestAccessTokenFromEnvironment(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "")
	t.Setenv("Q_CLI_ACCESS_TOKEN", "")
	if accessTokenFromEnvironment() {
		t.Fatal("no environment token expected")
	}
	t.Setenv("Q_CLI_ACCESS_TOKEN", "token")
	if !accessTokenFromEnvironment() {
		t.Fatal("Q_CLI_ACCESS_TOKEN must count as an environment token")
	}
	t.Setenv("Q_CLI_ACCESS_TOKEN", "")
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "token")
	if !accessTokenFromEnvironment() {
		t.Fatal("QOVERY_CLI_ACCESS_TOKEN must count as an environment token")
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
	mutex           sync.Mutex
	updateErrors    map[string]error
	historyFailures int
	fleetFailures   int
	historyBlocks   bool
	blockedHistory  map[string]bool
	fleetBlocks     bool
	historyStatus   qovery.StateEnum
	deployedAfter   int
	historyCalls    int
	reportedImages  map[string]string
	updated         []string
	cancelAfter     int
	cancel          context.CancelFunc
	calls           int
}

func (api *fakeClusterOperatorUpdateAPI) countCall() {
	api.mutex.Lock()
	defer api.mutex.Unlock()
	api.calls++
	if api.cancel != nil && api.calls == api.cancelAfter {
		api.cancel()
	}
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

func (api *fakeClusterOperatorUpdateAPI) fleet(ctx context.Context) ([]qovery.ClusterOperatorFleetInventoryResponse, error) {
	api.countCall()
	if api.fleetBlocks {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	api.mutex.Lock()
	defer api.mutex.Unlock()
	if api.fleetFailures > 0 {
		api.fleetFailures--
		return nil, errors.New("fleet unavailable")
	}
	fleet := make([]qovery.ClusterOperatorFleetInventoryResponse, 0, len(api.reportedImages))
	for clusterID, image := range api.reportedImages {
		cluster := fleetEntry(clusterID, qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT)
		cluster.SetReportedImageVersion(image)
		fleet = append(fleet, cluster)
	}
	return fleet, nil
}

func (api *fakeClusterOperatorUpdateAPI) history(ctx context.Context, _ string, clusterID string) ([]qovery.ClusterDeploymentHistory, error) {
	api.countCall()
	if api.historyBlocks || api.blockedHistory[clusterID] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	api.mutex.Lock()
	defer api.mutex.Unlock()
	if api.historyFailures > 0 {
		api.historyFailures--
		return nil, errors.New("history unavailable")
	}
	api.historyCalls++
	if api.deployedAfter > 0 && api.historyCalls > api.deployedAfter {
		return []qovery.ClusterDeploymentHistory{historyEntry("exec-"+clusterID, qovery.STATEENUM_DEPLOYED)}, nil
	}
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
	return clusterOperatorUpdateOptions{
		parallelRuns: 1,
		refreshDelay: time.Millisecond,
		timeout:      timeout,
		pollTimeout:  time.Second,
		expiryGrace:  time.Second,
		print:        func(string) {},
	}
}

func TestRunClusterOperatorUpdatesSucceedsWhenTargetIsReported(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYED, reportedImages: map[string]string{"c1": "v3", "c2": "v3"}}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1", "c2"), testUpdateOptions(time.Minute))

	for _, result := range results {
		if !result.started || result.err != nil || result.executionID != "exec-"+result.cluster.ClusterId {
			t.Fatalf("unexpected result %#v", result)
		}
	}
}

func TestRunClusterOperatorUpdatesFailsOnHistoryError(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYMENT_ERROR, reportedImages: map[string]string{"c1": "v3"}}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1"), testUpdateOptions(time.Minute))

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

func TestRunClusterOperatorUpdatesTimeoutForgetsRecoveredPollError(t *testing.T) {
	cases := []struct {
		name     string
		api      *fakeClusterOperatorUpdateAPI
		expected string
	}{
		{
			name:     "history recovered",
			api:      &fakeClusterOperatorUpdateAPI{historyFailures: 1, historyStatus: qovery.STATEENUM_DEPLOYING, cancelAfter: 3},
			expected: "timed out after 1m0s waiting for execution exec-c1",
		},
		{
			name:     "history cut by the timeout",
			api:      &fakeClusterOperatorUpdateAPI{historyBlocks: true, cancelAfter: 1},
			expected: "timed out after 1m0s waiting for execution exec-c1",
		},
		{
			name:     "fleet recovered",
			api:      &fakeClusterOperatorUpdateAPI{fleetFailures: 1, historyStatus: qovery.STATEENUM_DEPLOYED, reportedImages: map[string]string{"c1": "v2"}, cancelAfter: 3},
			expected: "timed out after 1m0s waiting for the Operator to report the target version",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			testCase.api.cancel = cancel

			results := runClusterOperatorUpdates(ctx, testCase.api, dockerUpdates("c1"), testUpdateOptions(time.Minute))

			if results[0].err == nil || results[0].err.Error() != testCase.expected {
				t.Fatalf("expected %q, got %v", testCase.expected, results[0].err)
			}
		})
	}
}

func TestPollClusterOperatorUpdateBoundsFinalVerification(t *testing.T) {
	cases := []struct {
		name              string
		api               *fakeClusterOperatorUpdateAPI
		executionFinished bool
	}{
		{name: "history hangs", api: &fakeClusterOperatorUpdateAPI{historyBlocks: true}},
		{name: "fleet hangs", api: &fakeClusterOperatorUpdateAPI{fleetBlocks: true}, executionFinished: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			expiredContext, cancel := context.WithCancel(context.Background())
			cancel()
			run := &clusterOperatorUpdateRun{
				cluster:           fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE),
				response:          *qovery.NewClusterOperatorUpdateResponse("exec-c1", "v3"),
				ctx:               expiredContext,
				cancel:            cancel,
				startedAt:         time.Now(),
				executionFinished: testCase.executionFinished,
			}
			options := testUpdateOptions(time.Minute)
			options.pollTimeout = 3 * time.Second
			options.expiryGrace = 30 * time.Millisecond

			started := time.Now()
			done, err := pollClusterOperatorUpdate(context.Background(), testCase.api, run, &clusterOperatorFleetSnapshot{}, options)

			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("final verification took %s, expected the grace period", elapsed)
			}
			if !done || err == nil || !strings.Contains(err.Error(), "timed out") {
				t.Fatalf("expected timeout, got done=%t err=%v", done, err)
			}
		})
	}
}

func TestClusterOperatorUpdateTimeoutError(t *testing.T) {
	withoutPollError := clusterOperatorUpdateTimeoutError(time.Minute, "exec-1", false, nil).Error()
	if withoutPollError != "timed out after 1m0s waiting for execution exec-1" {
		t.Fatalf("unexpected message %q", withoutPollError)
	}
	withPollError := clusterOperatorUpdateTimeoutError(time.Minute, "exec-1", true, errors.New("503")).Error()
	if withPollError != "timed out after 1m0s waiting for the Operator to report the target version (last poll error: 503)" {
		t.Fatalf("unexpected message %q", withPollError)
	}
}

func TestRunClusterOperatorUpdatesStopsAfterFirstFailure(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{
		updateErrors:   map[string]error{"c2": errors.New("412 Precondition Failed")},
		historyStatus:  qovery.STATEENUM_DEPLOYED,
		reportedImages: map[string]string{"c1": "v3", "c2": "v3", "c3": "v3", "c4": "v3"},
	}

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1", "c2", "c3", "c4"), testUpdateOptions(time.Minute))

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

func TestOperatorCommandsRejectArguments(t *testing.T) {
	for _, command := range []*cobra.Command{adminClusterOperatorCmd, clusterOperatorCmd} {
		if err := command.ValidateArgs([]string{"typo"}); err == nil {
			t.Fatalf("%s must reject unknown arguments", command.CommandPath())
		}
		if err := command.ValidateArgs(nil); err != nil {
			t.Fatalf("%s must accept no arguments: %v", command.CommandPath(), err)
		}
	}
}

func TestClusterOperatorLogPrefix(t *testing.T) {
	cluster := fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT)
	cases := []struct {
		name     string
		names    map[string]string
		expected string
	}{
		{name: "known organization", names: map[string]string{"org-c1": "Acme"}, expected: "[Organization 'Acme' - Cluster 'name-c1']"},
		{name: "unknown organization", names: map[string]string{"org-other": "Other"}, expected: "[Organization 'org-c1' - Cluster 'name-c1']"},
		{name: "empty name", names: map[string]string{"org-c1": ""}, expected: "[Organization 'org-c1' - Cluster 'name-c1']"},
		{name: "names unavailable", names: nil, expected: "[Organization 'org-c1' - Cluster 'name-c1']"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := clusterOperatorLogPrefix(cluster, testCase.names); got != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, got)
			}
		})
	}
}

func TestClusterOperatorUpdateProgressLine(t *testing.T) {
	kubernetes := fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE)
	docker := fleetEntry("c2", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE)
	kubernetesResponse := qovery.NewClusterOperatorUpdateResponse("exec-1", "v2")
	kubernetesResponse.SetChartVersion("0.2.0")
	dockerResponse := qovery.NewClusterOperatorUpdateResponse("exec-2", "v2")
	reportedKubernetes := operatorCluster(qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, true, qovery.CLUSTEROPERATORFLEETSTATUS_OUTDATED_IMAGE, "v1", "0.2.0", "0.2.0")
	reportedDocker := operatorCluster(qovery.CLUSTEROPERATORRUNTIME_DOCKER, false, qovery.CLUSTEROPERATORFLEETSTATUS_DISCONNECTED, "v1", "", "")

	cases := []struct {
		name     string
		run      clusterOperatorUpdateRun
		expected string
	}{
		{
			name:     "not recorded yet",
			run:      clusterOperatorUpdateRun{cluster: kubernetes, response: *kubernetesResponse},
			expected: "[P] - execution exec-1 not in the deployment history yet - 1m30s elapsed",
		},
		{
			name:     "queued",
			run:      clusterOperatorUpdateRun{cluster: kubernetes, response: *kubernetesResponse, executionStatus: qovery.STATEENUM_DEPLOYMENT_QUEUED},
			expected: "[P] - execution exec-1 DEPLOYMENT_QUEUED - 1m30s elapsed",
		},
		{
			name:     "deploying with poll error",
			run:      clusterOperatorUpdateRun{cluster: kubernetes, response: *kubernetesResponse, executionStatus: qovery.STATEENUM_DEPLOYING, lastPollError: errors.New("503")},
			expected: "[P] - execution exec-1 DEPLOYING - 1m30s elapsed - last poll error: 503",
		},
		{
			name:     "kubernetes heartbeat",
			run:      clusterOperatorUpdateRun{cluster: kubernetes, response: *kubernetesResponse, executionFinished: true, reported: &reportedKubernetes},
			expected: "[P] - waiting for the Operator heartbeat: reports v1/0.2.0, target v2/0.2.0 - 1m30s elapsed",
		},
		{
			name:     "heartbeat before first fleet poll",
			run:      clusterOperatorUpdateRun{cluster: kubernetes, response: *kubernetesResponse, executionFinished: true},
			expected: "[P] - waiting for the Operator heartbeat: reports unknown, target v2/0.2.0 - 1m30s elapsed",
		},
		{
			name:     "docker heartbeat disconnected",
			run:      clusterOperatorUpdateRun{cluster: docker, response: *dockerResponse, executionFinished: true, reported: &reportedDocker},
			expected: "[P] - waiting for the Operator heartbeat: reports v1 (disconnected), target v2 - 1m30s elapsed",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.run.progressLine("[P]", 90*time.Second+400*time.Millisecond); got != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, got)
			}
		})
	}
}

func TestClusterOperatorUpdateParameters(t *testing.T) {
	options := clusterOperatorUpdateOptions{parallelRuns: 2, refreshDelay: 30 * time.Second, timeout: 15 * time.Minute}

	defaults := strings.Join(clusterOperatorUpdateParameters(options, clusterOperatorUpdateTarget{}), "\n")
	explicit := strings.Join(clusterOperatorUpdateParameters(options, clusterOperatorUpdateTarget{imageVersion: "v1.3.0", chartVersion: "0.3.0"}), "\n")

	for _, expected := range []string{"- DryRunDisabled: true", "- ParallelRun: 2", "- RefreshDelay: 30 seconds", "- Timeout: 15m0s", "- ImageVersion: q-core target", "- ChartVersion: desired chart (KUBERNETES only)"} {
		if !strings.Contains(defaults, expected) {
			t.Fatalf("missing %q in %s", expected, defaults)
		}
	}
	if !strings.Contains(explicit, "- ImageVersion: v1.3.0") || !strings.Contains(explicit, "- ChartVersion: 0.3.0") {
		t.Fatalf("unexpected parameters %s", explicit)
	}
}

func TestLookupOrganizationNames(t *testing.T) {
	fleet := []qovery.ClusterOperatorFleetInventoryResponse{
		fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT),
		fleetEntry("c2", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT),
		fleetEntry("c3", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT),
	}
	fleet[1].OrganizationId = "org-c1"
	var lookedUp []string
	names := lookupOrganizationNames(context.Background(), clusterOrganizationIDs(fleet), func(ctx context.Context, organizationID string) (string, error) {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			t.Error("organization lookup must be bounded")
		}
		lookedUp = append(lookedUp, organizationID)
		if organizationID == "org-c3" {
			return "", errors.New("403 Forbidden")
		}
		return "Acme", nil
	}, time.Second, time.Second)

	if strings.Join(lookedUp, ",") != "org-c1,org-c3" {
		t.Fatalf("expected one lookup per distinct organization, got %v", lookedUp)
	}
	if len(names) != 1 || names["org-c1"] != "Acme" {
		t.Fatalf("unexpected names %v", names)
	}
}

func TestLookupOrganizationNamesStopsAtOverallDeadline(t *testing.T) {
	organizationIDs := make([]string, 0, 20)
	for index := 0; index < 20; index++ {
		organizationIDs = append(organizationIDs, fmt.Sprintf("org-%d", index))
	}
	started := time.Now()
	names := lookupOrganizationNames(context.Background(), organizationIDs, func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}, 50*time.Millisecond, 30*time.Millisecond)

	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("lookup took %s, expected to stop at the overall deadline", elapsed)
	}
	if len(names) != 0 {
		t.Fatalf("expected no names, got %v", names)
	}
	cluster := fleetEntry("c1", qovery.CLUSTEROPERATORRUNTIME_DOCKER, true, qovery.CLUSTEROPERATORFLEETSTATUS_CURRENT)
	if got := clusterOperatorLogPrefix(cluster, names); got != "[Organization 'org-c1' - Cluster 'name-c1']" {
		t.Fatalf("expected ID fallback, got %q", got)
	}
}

func TestAdminOrganizationNameUsesPublicRouteWithoutReauthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing authorization header")
		}
		switch request.URL.Path {
		case "/organization/org-1":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(writer, `{"id":"org-1","created_at":"2026-01-01T00:00:00Z","name":"Acme","plan":"ENTERPRISE"}`)
		default:
			writer.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	t.Setenv("QOVERY_API_URL", server.URL)
	reauthentications := 0
	api := &adminClusterOperatorUpdateAPI{credentials: &adminCredentials{
		tokenType:               "Bearer",
		token:                   "token",
		browserReauthentication: true,
		reload: func() (utils.AccessTokenType, utils.AccessToken, error) {
			return "Bearer", "token", nil
		},
		reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
			reauthentications++
			return "Bearer", "fresh", nil
		},
	}}

	name, err := api.organizationName(context.Background(), "org-1")
	if err != nil || name != "Acme" {
		t.Fatalf("unexpected name %q (err %v)", name, err)
	}
	if _, err := api.organizationName(context.Background(), "org-2"); err == nil {
		t.Fatal("expected an error on 401")
	}
	if reauthentications != 0 {
		t.Fatalf("organization lookups must not re-authenticate, got %d", reauthentications)
	}
}

func TestRunClusterOperatorUpdatesEvaluatesExpiredClusterBeforeTimingOut(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{
		blockedHistory: map[string]bool{"c1": true},
		historyStatus:  qovery.STATEENUM_DEPLOYED,
		reportedImages: map[string]string{"c2": "v3"},
	}
	options := testUpdateOptions(100 * time.Millisecond)
	options.parallelRuns = 2

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1", "c2"), options)

	if results[0].err == nil || !strings.Contains(results[0].err.Error(), "timed out") {
		t.Fatalf("expected c1 to time out, got %v", results[0].err)
	}
	if results[1].err != nil {
		t.Fatalf("c2 reached its target and must succeed, got %v", results[1].err)
	}
}

func TestRunClusterOperatorUpdatesBoundsEachHistoryCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	api := &fakeClusterOperatorUpdateAPI{blockedHistory: map[string]bool{"c1": true}, cancelAfter: 3, cancel: cancel}
	options := testUpdateOptions(time.Minute)
	options.pollTimeout = 20 * time.Millisecond
	recorder := &lineRecorder{}
	options.print = recorder.print

	runClusterOperatorUpdates(ctx, api, dockerUpdates("c1"), options)

	pollErrors := 0
	for _, line := range recorder.lines {
		if strings.HasSuffix(line, "elapsed - last poll error: context deadline exceeded") {
			pollErrors++
		}
	}
	if pollErrors != 2 {
		t.Fatalf("expected two bounded history calls before cancellation, got:\n%s", strings.Join(recorder.lines, "\n"))
	}
}

type lineRecorder struct {
	mutex sync.Mutex
	lines []string
}

func (recorder *lineRecorder) print(line string) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.lines = append(recorder.lines, line)
}

func TestRunClusterOperatorUpdatesReportsProgress(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYED, reportedImages: map[string]string{"c1": "v3"}}
	recorder := &lineRecorder{}
	options := testUpdateOptions(time.Minute)
	options.organizationNames = map[string]string{"org-c1": "Acme"}
	options.print = recorder.print

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1"), options)

	if results[0].err != nil {
		t.Fatal(results[0].err)
	}
	expected := []string{
		"[Organization 'Acme' - Cluster 'name-c1'] - Starting Operator update unknown -> v3 - execution exec-c1 - https://console.qovery.com/organization/org-c1/cluster/c1/cluster-logs",
		"Checking Operator updates in 0 seconds",
		"[Organization 'Acme' - Cluster 'name-c1'] execution exec-c1 finished, waiting for the Operator to report the target version",
		"[Organization 'Acme' - Cluster 'name-c1'] Operator reports image v3",
	}
	if strings.Join(recorder.lines, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected output:\n%s", strings.Join(recorder.lines, "\n"))
	}
}

func TestRunClusterOperatorUpdatesReportsExecutionState(t *testing.T) {
	api := &fakeClusterOperatorUpdateAPI{historyStatus: qovery.STATEENUM_DEPLOYING, deployedAfter: 2, reportedImages: map[string]string{"c1": "v3"}}
	recorder := &lineRecorder{}
	options := testUpdateOptions(time.Minute)
	options.print = recorder.print

	results := runClusterOperatorUpdates(context.Background(), api, dockerUpdates("c1"), options)

	if results[0].err != nil {
		t.Fatal(results[0].err)
	}
	progress := 0
	for _, line := range recorder.lines {
		if strings.HasPrefix(line, "[Organization 'org-c1' - Cluster 'name-c1'] - execution exec-c1 DEPLOYING - ") {
			progress++
		}
	}
	if progress != 2 {
		t.Fatalf("expected DEPLOYING progress lines, got:\n%s", strings.Join(recorder.lines, "\n"))
	}
}

func TestClusterOperatorUpdateRefreshDelayDefault(t *testing.T) {
	if got := adminClusterOperatorUpdateCmd.Flags().Lookup("refresh-delay").DefValue; got != "30" {
		t.Fatalf("expected default refresh delay 30, got %s", got)
	}
}
