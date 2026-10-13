package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	adminClusterOperatorUpdateFilters      map[string]string
	adminClusterOperatorUpdateImageVersion string
	adminClusterOperatorUpdateChartVersion string
	adminClusterOperatorUpdateExecute      bool
	adminClusterOperatorUpdateNoConfirm    bool
	adminClusterOperatorUpdateParallelRuns int
	adminClusterOperatorUpdateRefreshDelay int
	adminClusterOperatorUpdateTimeout      time.Duration
)

var adminClusterOperatorUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the Qovery Operator on attached clusters",
	Long: `Update the Qovery Operator image (and Helm chart for Kubernetes) on attached clusters.

The selection is the attached Operator fleet, narrowed with the filters below.
Without filters the whole attached fleet is considered.

A cluster is updated only when its Operator is connected, its runtime is KUBERNETES or DOCKER, and there is something to do:
* without --image-version / --chart-version: its status is OUTDATED_IMAGE, OUTDATED_CHART or OUTDATED_IMAGE_AND_CHART;
* with explicit versions: its reported image (or chart, for KUBERNETES) differs from the version the update would install.
Without --image-version q-core picks its target image. A KUBERNETES cluster without --chart-version gets its desired chart.
DOCKER clusters never receive a chart version.

The command runs in dry run by default and only prints the plan. Use --disable-dry-run (-y) to trigger the updates.
Up to --parallel-run updates run at the same time. After the first failure no new update is started.

` + clusterOperatorFilterHelp() + `
Examples:
qovery admin cluster operator update
qovery admin cluster operator update -f Runtime=DOCKER -y
qovery admin cluster operator update -f ClusterId=<cluster-id> --image-version=v1.204.0 --chart-version=0.3.0 -y
qovery admin cluster operator update -f Status=OUTDATED_IMAGE -n 5 --refresh-delay=30 --timeout=20m -y -c
qovery admin cluster operator update -f ClusterKind=EKS_ANYWHERE
qovery admin cluster operator update -f Runtime=DOCKER,KUBERNETES -f Connected=true`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		options, err := newClusterOperatorUpdateOptions(
			adminClusterOperatorUpdateParallelRuns,
			adminClusterOperatorUpdateRefreshDelay,
			adminClusterOperatorUpdateTimeout,
		)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		if _, err := parseClusterOperatorFilters(adminClusterOperatorUpdateFilters); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		api := &adminClusterOperatorUpdateAPI{
			adminURL:   utils.GetAdminUrl(),
			httpClient: &http.Client{Timeout: 60 * time.Second},
			credentials: &adminCredentials{
				tokenType:               tokenType,
				token:                   token,
				browserReauthentication: !accessTokenFromEnvironment(),
				reload: func() (utils.AccessTokenType, utils.AccessToken, error) {
					return utils.GetAccessToken(false)
				},
				reauthenticate: func() (utils.AccessTokenType, utils.AccessToken, error) {
					pkg.DoRequestUserToAuthenticate(false, true)
					return utils.GetAccessToken(false)
				},
			},
		}

		ctx := context.Background()
		fleet, err := api.fleet(ctx)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		clusters, err := selectClusterOperators(fleet, adminClusterOperatorUpdateFilters, false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		sortClusterOperators(clusters)
		organizationNames := lookupOrganizationNames(ctx, clusterOrganizationIDs(clusters), api.organizationName, 15*time.Second, 10*time.Second)
		options.organizationNames = organizationNames

		target := clusterOperatorUpdateTarget{
			imageVersion: adminClusterOperatorUpdateImageVersion,
			chartVersion: adminClusterOperatorUpdateChartVersion,
		}
		plan := planClusterOperatorUpdates(clusters, target)
		if err := utils.PrintTable(
			[]string{"Organization", "Cluster ID", "Cluster", "Kind", "Runtime", "Image", "Chart", "Action"},
			clusterOperatorUpdatePlanRows(plan, target.imageVersion, target.chartVersion, organizationNames),
		); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		updates := eligibleClusterOperatorUpdates(plan)
		utils.Println(fmt.Sprintf("%d cluster(s) to update, %d skipped", len(updates), len(plan)-len(updates)))
		if len(updates) == 0 {
			return
		}
		if !adminClusterOperatorUpdateExecute {
			utils.Println("Dry run: no update triggered. Use --disable-dry-run (-y) to execute.")
			return
		}
		for _, line := range clusterOperatorUpdateParameters(options, target) {
			utils.Println(line)
		}
		if !adminClusterOperatorUpdateNoConfirm {
			utils.Println("Do you want to continue the Operator update?")
			if !utils.Validate("operator update") {
				utils.Println("Exiting: Validation failed")
				return
			}
		}

		results := runClusterOperatorUpdates(ctx, api, updates, options)
		failed, err := printClusterOperatorUpdateSummary(results, organizationNames)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		if failed {
			os.Exit(1)
		}
	},
}

type clusterOperatorUpdateTarget struct {
	imageVersion string
	chartVersion string
}

func (target clusterOperatorUpdateTarget) explicit() bool {
	return target.imageVersion != "" || target.chartVersion != ""
}

type clusterOperatorUpdatePlanItem struct {
	cluster    qovery.ClusterOperatorFleetInventoryResponse
	request    *qovery.ClusterOperatorUpdateRequest
	skipReason string
}

func planClusterOperatorUpdates(
	clusters []qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) []clusterOperatorUpdatePlanItem {
	plan := make([]clusterOperatorUpdatePlanItem, 0, len(clusters))
	for _, cluster := range clusters {
		request, skipReason := clusterOperatorUpdateEligibility(cluster, target)
		plan = append(plan, clusterOperatorUpdatePlanItem{cluster: cluster, request: request, skipReason: skipReason})
	}
	return plan
}

func clusterOperatorUpdateEligibility(
	cluster qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) (*qovery.ClusterOperatorUpdateRequest, string) {
	if !cluster.Connected {
		return nil, "operator disconnected"
	}
	switch cluster.ReportedRuntime {
	case qovery.CLUSTEROPERATORRUNTIME_KUBERNETES, qovery.CLUSTEROPERATORRUNTIME_DOCKER:
	default:
		return nil, fmt.Sprintf("runtime %s not supported", displayRuntime(cluster.ReportedRuntime))
	}

	request := buildClusterOperatorUpdateRequest(cluster, target)
	if cluster.ReportedRuntime == qovery.CLUSTEROPERATORRUNTIME_KUBERNETES && !request.ChartVersion.IsSet() {
		return nil, "no target chart version"
	}

	if !target.explicit() {
		if !isClusterOperatorOutdated(cluster.Status) {
			return nil, fmt.Sprintf("status %s", cluster.Status)
		}
		return request, ""
	}
	if cluster.ReportedRuntime == qovery.CLUSTEROPERATORRUNTIME_DOCKER && target.imageVersion == "" {
		return nil, "chart version does not apply to DOCKER"
	}
	if cluster.ReportedRuntime == qovery.CLUSTEROPERATORRUNTIME_DOCKER &&
		!dockerHeartbeatCanConfirmImage(target.imageVersion, cluster.DesiredImageVersion) {
		return nil, "DOCKER heartbeat cannot confirm a non-release image tag"
	}
	if !clusterOperatorDiffersFromTarget(cluster, target) {
		return nil, "already on requested version"
	}
	return request, ""
}

var releaseImageVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

func dockerHeartbeatCanConfirmImage(imageVersion string, desiredImageVersion qovery.NullableString) bool {
	return imageVersion == nullableStringValue(desiredImageVersion) || releaseImageVersionPattern.MatchString(imageVersion)
}

func clusterOperatorDiffersFromTarget(
	cluster qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) bool {
	if imageVersion := effectiveImageVersion(cluster, target); imageVersion != "" && nullableStringValue(cluster.ReportedImageVersion) != imageVersion {
		return true
	}
	chartVersion := effectiveChartVersion(cluster, target)
	return chartVersion != "" && nullableStringValue(cluster.ReportedChartVersion) != chartVersion
}

func effectiveImageVersion(
	cluster qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) string {
	if target.imageVersion != "" {
		return target.imageVersion
	}
	return nullableStringValue(cluster.DesiredImageVersion)
}

func effectiveChartVersion(
	cluster qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) string {
	if cluster.ReportedRuntime != qovery.CLUSTEROPERATORRUNTIME_KUBERNETES {
		return ""
	}
	if target.chartVersion != "" {
		return target.chartVersion
	}
	return nullableStringValue(cluster.DesiredChartVersion)
}

func buildClusterOperatorUpdateRequest(
	cluster qovery.ClusterOperatorFleetInventoryResponse,
	target clusterOperatorUpdateTarget,
) *qovery.ClusterOperatorUpdateRequest {
	request := qovery.NewClusterOperatorUpdateRequest()
	if target.imageVersion != "" {
		request.SetImageVersion(target.imageVersion)
	}
	if chartVersion := effectiveChartVersion(cluster, target); chartVersion != "" {
		request.SetChartVersion(chartVersion)
	}
	return request
}

func eligibleClusterOperatorUpdates(plan []clusterOperatorUpdatePlanItem) []clusterOperatorUpdatePlanItem {
	eligible := make([]clusterOperatorUpdatePlanItem, 0, len(plan))
	for _, item := range plan {
		if item.request != nil {
			eligible = append(eligible, item)
		}
	}
	return eligible
}

func clusterOperatorUpdatePlanRows(
	plan []clusterOperatorUpdatePlanItem,
	imageVersion string,
	chartVersion string,
	organizationNames map[string]string,
) [][]string {
	rows := make([][]string, 0, len(plan))
	for _, item := range plan {
		cluster := item.cluster
		targetImage := imageVersion
		if targetImage == "" {
			targetImage = displayVersion(cluster.DesiredImageVersion)
		}
		chart := "-"
		if cluster.ReportedRuntime != qovery.CLUSTEROPERATORRUNTIME_DOCKER {
			targetChart := chartVersion
			if targetChart == "" {
				targetChart = displayVersion(cluster.DesiredChartVersion)
			}
			chart = displayVersion(cluster.ReportedChartVersion) + " -> " + targetChart
		}
		action := "update"
		if item.request == nil {
			action = "skip: " + item.skipReason
		}
		rows = append(rows, []string{
			organizationDisplayName(organizationNames, cluster.OrganizationId),
			cluster.ClusterId,
			cluster.ClusterName,
			string(cluster.ClusterKind),
			displayRuntime(cluster.ReportedRuntime),
			displayVersion(cluster.ReportedImageVersion) + " -> " + targetImage,
			chart,
			action,
		})
	}
	return rows
}

func newClusterOperatorUpdateOptions(parallelRuns int, refreshDelaySeconds int, timeout time.Duration) (clusterOperatorUpdateOptions, error) {
	if parallelRuns < 1 || parallelRuns > 10 {
		return clusterOperatorUpdateOptions{}, fmt.Errorf("--parallel-run must be between 1 and 10, got %d", parallelRuns)
	}
	if refreshDelaySeconds < 5 || refreshDelaySeconds > 120 {
		return clusterOperatorUpdateOptions{}, fmt.Errorf("--refresh-delay must be between 5 and 120 seconds, got %d", refreshDelaySeconds)
	}
	if timeout <= 0 {
		return clusterOperatorUpdateOptions{}, fmt.Errorf("--timeout must be positive, got %s", timeout)
	}
	return clusterOperatorUpdateOptions{
		parallelRuns: parallelRuns,
		refreshDelay: time.Duration(refreshDelaySeconds) * time.Second,
		timeout:      timeout,
		pollTimeout:  60 * time.Second,
		expiryGrace:  10 * time.Second,
		print:        utils.Println,
	}, nil
}

type clusterOperatorUpdateOptions struct {
	parallelRuns      int
	refreshDelay      time.Duration
	timeout           time.Duration
	pollTimeout       time.Duration
	expiryGrace       time.Duration
	organizationNames map[string]string
	print             func(string)
}

type clusterOperatorUpdateAPI interface {
	update(ctx context.Context, clusterID string, request qovery.ClusterOperatorUpdateRequest) (*qovery.ClusterOperatorUpdateResponse, error)
	fleet(ctx context.Context) ([]qovery.ClusterOperatorFleetInventoryResponse, error)
	history(ctx context.Context, organizationID string, clusterID string) ([]qovery.ClusterDeploymentHistory, error)
}

type clusterOperatorUpdateResult struct {
	cluster     qovery.ClusterOperatorFleetInventoryResponse
	started     bool
	executionID string
	err         error
}

type clusterOperatorUpdateRun struct {
	index             int
	cluster           qovery.ClusterOperatorFleetInventoryResponse
	response          qovery.ClusterOperatorUpdateResponse
	ctx               context.Context
	cancel            context.CancelFunc
	startedAt         time.Time
	executionStatus   qovery.StateEnum
	executionFinished bool
	reported          *qovery.ClusterOperatorFleetInventoryResponse
	lastPollError     error
}

type clusterOperatorFleetSnapshot struct {
	fetched  bool
	clusters []qovery.ClusterOperatorFleetInventoryResponse
	err      error
}

func runClusterOperatorUpdates(
	ctx context.Context,
	api clusterOperatorUpdateAPI,
	updates []clusterOperatorUpdatePlanItem,
	options clusterOperatorUpdateOptions,
) []clusterOperatorUpdateResult {
	results := make([]clusterOperatorUpdateResult, len(updates))
	for index, item := range updates {
		results[index].cluster = item.cluster
	}
	finish := func(run *clusterOperatorUpdateRun, err error) {
		run.cancel()
		results[run.index].err = err
		if err != nil {
			options.print(fmt.Sprintf("%s failed: %s", clusterOperatorLogPrefix(run.cluster, options.organizationNames), err))
		}
	}

	var running []*clusterOperatorUpdateRun
	next := 0
	failed := false
	for {
		for !failed && next < len(updates) && len(running) < options.parallelRuns {
			index := next
			next++
			results[index].started = true
			run, err := startClusterOperatorUpdate(ctx, api, index, updates[index], options)
			if err != nil {
				failed = true
				results[index].err = err
				options.print(fmt.Sprintf("%s failed: %s", clusterOperatorLogPrefix(updates[index].cluster, options.organizationNames), err))
				continue
			}
			results[index].executionID = run.response.ExecutionId
			running = append(running, run)
		}
		if len(running) == 0 {
			return results
		}

		options.print(fmt.Sprintf("Checking Operator updates in %d seconds", int(options.refreshDelay.Seconds())))
		time.Sleep(options.refreshDelay)

		snapshot := &clusterOperatorFleetSnapshot{}
		stillRunning := running[:0]
		for _, run := range running {
			done, err := pollClusterOperatorUpdate(ctx, api, run, snapshot, options)
			if !done {
				stillRunning = append(stillRunning, run)
				continue
			}
			finish(run, err)
			if err != nil {
				failed = true
			}
		}
		running = stillRunning
	}
}

func startClusterOperatorUpdate(
	ctx context.Context,
	api clusterOperatorUpdateAPI,
	index int,
	item clusterOperatorUpdatePlanItem,
	options clusterOperatorUpdateOptions,
) (*clusterOperatorUpdateRun, error) {
	runContext, cancel := context.WithTimeout(ctx, options.timeout)
	cluster := item.cluster
	response, err := api.update(runContext, cluster.ClusterId, *item.request)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("update request failed: %w", err)
	}
	options.print(fmt.Sprintf(
		"%s - Starting Operator update %s -> %s - execution %s - https://console.qovery.com/organization/%s/cluster/%s/cluster-logs",
		clusterOperatorLogPrefix(cluster, options.organizationNames),
		clusterOperatorVersionPair(cluster.ReportedRuntime, nullableStringValue(cluster.ReportedImageVersion), nullableStringValue(cluster.ReportedChartVersion)),
		clusterOperatorVersionPair(cluster.ReportedRuntime, response.ImageVersion, nullableStringValue(response.ChartVersion)),
		response.ExecutionId,
		cluster.OrganizationId,
		cluster.ClusterId,
	))
	return &clusterOperatorUpdateRun{
		index:     index,
		cluster:   cluster,
		response:  *response,
		ctx:       runContext,
		cancel:    cancel,
		startedAt: time.Now(),
	}, nil
}

func pollClusterOperatorUpdate(
	ctx context.Context,
	api clusterOperatorUpdateAPI,
	run *clusterOperatorUpdateRun,
	snapshot *clusterOperatorFleetSnapshot,
	options clusterOperatorUpdateOptions,
) (bool, error) {
	prefix := clusterOperatorLogPrefix(run.cluster, options.organizationNames)
	expired := run.ctx.Err() != nil
	timeoutError := func() error {
		return clusterOperatorUpdateTimeoutError(options.timeout, run.response.ExecutionId, run.executionFinished, run.lastPollError)
	}
	historyParent := run.ctx
	fleetParent := ctx
	if expired {
		graceContext, cancelGrace := context.WithTimeout(ctx, options.expiryGrace)
		defer cancelGrace()
		historyParent = graceContext
		fleetParent = graceContext
	}

	if !run.executionFinished {
		historyContext, cancel := context.WithTimeout(historyParent, options.pollTimeout)
		history, err := api.history(historyContext, run.cluster.OrganizationId, run.cluster.ClusterId)
		cancel()
		if err != nil {
			if expired || run.ctx.Err() != nil {
				return true, timeoutError()
			}
			run.lastPollError = err
			options.print(run.progressLine(prefix, time.Since(run.startedAt)))
			return false, nil
		}
		run.lastPollError = nil
		if status, found := clusterOperatorExecutionStatus(history, run.response.ExecutionId); found {
			run.executionStatus = status
		}
		finished, err := evaluateClusterOperatorUpdateExecution(history, run.response.ExecutionId)
		if err != nil {
			return true, err
		}
		if !finished {
			if expired {
				return true, timeoutError()
			}
			options.print(run.progressLine(prefix, time.Since(run.startedAt)))
			return false, nil
		}
		run.executionFinished = true
		options.print(fmt.Sprintf("%s execution %s finished, waiting for the Operator to report the target version", prefix, run.response.ExecutionId))
	}

	if !snapshot.fetched {
		fleetContext, cancel := context.WithTimeout(fleetParent, options.pollTimeout)
		snapshot.clusters, snapshot.err = api.fleet(fleetContext)
		cancel()
		snapshot.fetched = true
	}
	if snapshot.err != nil {
		run.lastPollError = snapshot.err
		if expired || run.ctx.Err() != nil {
			return true, timeoutError()
		}
		options.print(run.progressLine(prefix, time.Since(run.startedAt)))
		return false, nil
	}
	run.lastPollError = nil
	run.reported = findClusterOperator(snapshot.clusters, run.cluster.ClusterId)
	if clusterOperatorReportsTarget(snapshot.clusters, run.cluster.ClusterId, run.response) {
		options.print(fmt.Sprintf("%s Operator reports %s", prefix, clusterOperatorUpdateVersions(run.response)))
		return true, nil
	}
	if expired || run.ctx.Err() != nil {
		return true, timeoutError()
	}
	options.print(run.progressLine(prefix, time.Since(run.startedAt)))
	return false, nil
}

func (run *clusterOperatorUpdateRun) progressLine(prefix string, elapsed time.Duration) string {
	elapsedText := elapsed.Round(time.Second).String()
	var line string
	switch {
	case run.executionFinished:
		reported := "unknown"
		if run.reported != nil {
			reported = clusterOperatorVersionPair(run.cluster.ReportedRuntime, nullableStringValue(run.reported.ReportedImageVersion), nullableStringValue(run.reported.ReportedChartVersion))
			if !run.reported.Connected {
				reported += " (disconnected)"
			}
		}
		target := clusterOperatorVersionPair(run.cluster.ReportedRuntime, run.response.ImageVersion, nullableStringValue(run.response.ChartVersion))
		line = fmt.Sprintf("%s - waiting for the Operator heartbeat: reports %s, target %s - %s elapsed", prefix, reported, target, elapsedText)
	case run.executionStatus == "":
		line = fmt.Sprintf("%s - execution %s not in the deployment history yet - %s elapsed", prefix, run.response.ExecutionId, elapsedText)
	default:
		line = fmt.Sprintf("%s - execution %s %s - %s elapsed", prefix, run.response.ExecutionId, run.executionStatus, elapsedText)
	}
	if run.lastPollError != nil {
		line += fmt.Sprintf(" - last poll error: %s", run.lastPollError)
	}
	return line
}

func clusterOperatorExecutionStatus(history []qovery.ClusterDeploymentHistory, executionID string) (qovery.StateEnum, bool) {
	for _, entry := range history {
		if nullableStringValue(entry.Identifier.ExecutionId) == executionID {
			return entry.Status, true
		}
	}
	return "", false
}

func findClusterOperator(fleet []qovery.ClusterOperatorFleetInventoryResponse, clusterID string) *qovery.ClusterOperatorFleetInventoryResponse {
	for index := range fleet {
		if fleet[index].ClusterId == clusterID {
			return &fleet[index]
		}
	}
	return nil
}

func clusterOperatorVersionPair(runtime qovery.ClusterOperatorRuntime, imageVersion string, chartVersion string) string {
	image := imageVersion
	if image == "" {
		image = "unknown"
	}
	if runtime != qovery.CLUSTEROPERATORRUNTIME_KUBERNETES {
		return image
	}
	if chartVersion == "" {
		chartVersion = "unknown"
	}
	return image + "/" + chartVersion
}

func clusterOperatorUpdateParameters(options clusterOperatorUpdateOptions, target clusterOperatorUpdateTarget) []string {
	imageVersion := target.imageVersion
	if imageVersion == "" {
		imageVersion = "q-core target"
	}
	chartVersion := target.chartVersion
	if chartVersion == "" {
		chartVersion = "desired chart (KUBERNETES only)"
	}
	return []string{
		"-------------------------------------------",
		"- DryRunDisabled: true",
		fmt.Sprintf("- ParallelRun: %d", options.parallelRuns),
		fmt.Sprintf("- RefreshDelay: %d seconds", int(options.refreshDelay.Seconds())),
		fmt.Sprintf("- Timeout: %s", options.timeout),
		fmt.Sprintf("- ImageVersion: %s", imageVersion),
		fmt.Sprintf("- ChartVersion: %s", chartVersion),
		"-------------------------------------------",
	}
}

func clusterOperatorUpdateTimeoutError(timeout time.Duration, executionID string, executionFinished bool, lastPollError error) error {
	message := fmt.Sprintf("timed out after %s waiting for execution %s", timeout, executionID)
	if executionFinished {
		message = fmt.Sprintf("timed out after %s waiting for the Operator to report the target version", timeout)
	}
	if lastPollError != nil {
		message += fmt.Sprintf(" (last poll error: %s)", lastPollError)
	}
	return errors.New(message)
}

func evaluateClusterOperatorUpdateExecution(history []qovery.ClusterDeploymentHistory, executionID string) (bool, error) {
	for _, entry := range history {
		if nullableStringValue(entry.Identifier.ExecutionId) != executionID {
			continue
		}
		if entry.Status == qovery.STATEENUM_DEPLOYED {
			return true, nil
		}
		if utils.IsTerminalState(entry.Status) {
			return true, fmt.Errorf("execution %s ended with status %s", executionID, entry.Status)
		}
		return false, nil
	}
	return false, nil
}

func clusterOperatorReportsTarget(
	fleet []qovery.ClusterOperatorFleetInventoryResponse,
	clusterID string,
	response qovery.ClusterOperatorUpdateResponse,
) bool {
	for _, cluster := range fleet {
		if cluster.ClusterId != clusterID {
			continue
		}
		if !cluster.Connected || nullableStringValue(cluster.ReportedImageVersion) != response.ImageVersion {
			return false
		}
		if cluster.ReportedRuntime == qovery.CLUSTEROPERATORRUNTIME_KUBERNETES {
			return nullableStringValue(cluster.ReportedChartVersion) == nullableStringValue(response.ChartVersion)
		}
		return true
	}
	return false
}

func printClusterOperatorUpdateSummary(results []clusterOperatorUpdateResult, organizationNames map[string]string) (bool, error) {
	failed := false
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		outcome := "succeeded"
		switch {
		case !result.started:
			outcome = "not started"
		case result.err != nil:
			failed = true
			outcome = "failed: " + result.err.Error()
		}
		rows = append(rows, []string{
			organizationDisplayName(organizationNames, result.cluster.OrganizationId),
			result.cluster.ClusterId,
			result.cluster.ClusterName,
			result.executionID,
			outcome,
		})
	}
	utils.Println("")
	return failed, utils.PrintTable([]string{"Organization", "Cluster ID", "Cluster", "Execution ID", "Result"}, rows)
}

func clusterOperatorUpdateVersions(response qovery.ClusterOperatorUpdateResponse) string {
	if chartVersion := nullableStringValue(response.ChartVersion); chartVersion != "" {
		return fmt.Sprintf("image %s, chart %s", response.ImageVersion, chartVersion)
	}
	return "image " + response.ImageVersion
}

func clusterOperatorLogPrefix(cluster qovery.ClusterOperatorFleetInventoryResponse, organizationNames map[string]string) string {
	return fmt.Sprintf("[Organization '%s' - Cluster '%s']", organizationDisplayName(organizationNames, cluster.OrganizationId), cluster.ClusterName)
}

func organizationDisplayName(organizationNames map[string]string, organizationID string) string {
	if name := organizationNames[organizationID]; name != "" {
		return name
	}
	return organizationID
}

func displayRuntime(runtime qovery.ClusterOperatorRuntime) string {
	if runtime == "" {
		return string(qovery.CLUSTEROPERATORRUNTIME_UNKNOWN)
	}
	return string(runtime)
}

func nullableStringValue(value qovery.NullableString) string {
	if pointer := value.Get(); pointer != nil {
		return *pointer
	}
	return ""
}

func postClusterOperatorUpdate(
	ctx context.Context,
	adminURL string,
	authorization string,
	httpClient *http.Client,
	clusterID string,
	updateRequest qovery.ClusterOperatorUpdateRequest,
) (*qovery.ClusterOperatorUpdateResponse, error) {
	body, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(adminURL, "/")+"/cluster/operator/update/"+url.PathEscape(clusterID),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusAccepted {
		return nil, newHTTPStatusError("operator update API", response)
	}
	var updateResponse qovery.ClusterOperatorUpdateResponse
	if err := json.NewDecoder(response.Body).Decode(&updateResponse); err != nil {
		return nil, err
	}
	return &updateResponse, nil
}

type adminCredentials struct {
	mutex                   sync.Mutex
	tokenType               utils.AccessTokenType
	token                   utils.AccessToken
	browserReauthentication bool
	browserReauthenticated  bool
	reload                  func() (utils.AccessTokenType, utils.AccessToken, error)
	reauthenticate          func() (utils.AccessTokenType, utils.AccessToken, error)
}

func accessTokenFromEnvironment() bool {
	return os.Getenv("QOVERY_CLI_ACCESS_TOKEN") != "" || os.Getenv("Q_CLI_ACCESS_TOKEN") != ""
}

func (credentials *adminCredentials) current() (utils.AccessTokenType, utils.AccessToken) {
	credentials.mutex.Lock()
	defer credentials.mutex.Unlock()
	return credentials.tokenType, credentials.token
}

func (credentials *adminCredentials) refresh(staleToken utils.AccessToken, unauthorized error) error {
	credentials.mutex.Lock()
	defer credentials.mutex.Unlock()
	if credentials.token != staleToken {
		return nil
	}
	if tokenType, token, err := credentials.reload(); err == nil && token != staleToken {
		credentials.tokenType = tokenType
		credentials.token = token
		return nil
	}
	if !credentials.browserReauthentication || credentials.browserReauthenticated {
		return unauthorized
	}
	credentials.browserReauthenticated = true
	utils.Println("Needs to re-authenticate as the response is UNAUTHORIZED (401)")
	tokenType, token, err := credentials.reauthenticate()
	if err != nil {
		return err
	}
	credentials.tokenType = tokenType
	credentials.token = token
	return nil
}

func withAdminReauthentication[T any](
	credentials *adminCredentials,
	call func(tokenType utils.AccessTokenType, token utils.AccessToken) (T, error),
) (T, error) {
	tokenType, token := credentials.current()
	result, err := call(tokenType, token)
	if !isUnauthorizedError(err) {
		return result, err
	}
	if err := credentials.refresh(token, err); err != nil {
		var zero T
		return zero, err
	}
	tokenType, token = credentials.current()
	return call(tokenType, token)
}

type adminClusterOperatorUpdateAPI struct {
	adminURL    string
	httpClient  *http.Client
	credentials *adminCredentials
}

func (api *adminClusterOperatorUpdateAPI) update(
	ctx context.Context,
	clusterID string,
	request qovery.ClusterOperatorUpdateRequest,
) (*qovery.ClusterOperatorUpdateResponse, error) {
	return withAdminReauthentication(api.credentials, func(tokenType utils.AccessTokenType, token utils.AccessToken) (*qovery.ClusterOperatorUpdateResponse, error) {
		return postClusterOperatorUpdate(ctx, api.adminURL, utils.GetAuthorizationHeaderValue(tokenType, token), api.httpClient, clusterID, request)
	})
}

func (api *adminClusterOperatorUpdateAPI) fleet(ctx context.Context) ([]qovery.ClusterOperatorFleetInventoryResponse, error) {
	return withAdminReauthentication(api.credentials, func(tokenType utils.AccessTokenType, token utils.AccessToken) ([]qovery.ClusterOperatorFleetInventoryResponse, error) {
		fleet, err := getClusterOperatorFleet(ctx, api.adminURL, utils.GetAuthorizationHeaderValue(tokenType, token), api.httpClient)
		if err != nil {
			return nil, err
		}
		return fleet.GetResults(), nil
	})
}

func (api *adminClusterOperatorUpdateAPI) organizationName(ctx context.Context, organizationID string) (string, error) {
	tokenType, token := api.credentials.current()
	return organizationNameLookup(tokenType, token)(ctx, organizationID)
}

func organizationNameLookup(
	tokenType utils.AccessTokenType,
	token utils.AccessToken,
) func(ctx context.Context, organizationID string) (string, error) {
	client := utils.GetQoveryClient(tokenType, token)
	return func(ctx context.Context, organizationID string) (string, error) {
		organization, _, err := client.OrganizationMainCallsAPI.GetOrganization(ctx, organizationID).Execute()
		if err != nil {
			return "", err
		}
		return organization.Name, nil
	}
}

func clusterOrganizationIDs(clusters []qovery.ClusterOperatorFleetInventoryResponse) []string {
	ids := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		if !slices.Contains(ids, cluster.OrganizationId) {
			ids = append(ids, cluster.OrganizationId)
		}
	}
	return ids
}

func lookupOrganizationNames(
	ctx context.Context,
	organizationIDs []string,
	lookup func(ctx context.Context, organizationID string) (string, error),
	overallTimeout time.Duration,
	callTimeout time.Duration,
) map[string]string {
	names := make(map[string]string, len(organizationIDs))
	overallContext, cancelOverall := context.WithTimeout(ctx, overallTimeout)
	defer cancelOverall()
	for _, organizationID := range organizationIDs {
		if overallContext.Err() != nil {
			break
		}
		lookupContext, cancel := context.WithTimeout(overallContext, callTimeout)
		name, err := lookup(lookupContext, organizationID)
		cancel()
		if err == nil && name != "" {
			names[organizationID] = name
		}
	}
	return names
}

func (api *adminClusterOperatorUpdateAPI) history(
	ctx context.Context,
	organizationID string,
	clusterID string,
) ([]qovery.ClusterDeploymentHistory, error) {
	return withAdminReauthentication(api.credentials, func(tokenType utils.AccessTokenType, token utils.AccessToken) ([]qovery.ClusterDeploymentHistory, error) {
		history, response, err := utils.GetQoveryClient(tokenType, token).ClusterDeploymentHistoryAPI.
			ListClusterDeploymentHistoryV2(ctx, organizationID, clusterID).
			PageSize(10).
			Execute()
		if response != nil && response.StatusCode == http.StatusUnauthorized {
			return nil, &httpStatusError{operation: "cluster deployment history API", statusCode: response.StatusCode, status: response.Status}
		}
		if err != nil {
			return nil, err
		}
		return history.GetResults(), nil
	})
}

func init() {
	adminClusterOperatorCmd.AddCommand(adminClusterOperatorUpdateCmd)
	flags := adminClusterOperatorUpdateCmd.Flags()
	flags.StringToStringVarP(&adminClusterOperatorUpdateFilters, "filters", "f", make(map[string]string), "Value(s) to filter the property selected separated by comma when multiple values are defined")
	flags.StringVar(&adminClusterOperatorUpdateImageVersion, "image-version", "", "Operator image version to install (default: q-core target)")
	flags.StringVar(&adminClusterOperatorUpdateChartVersion, "chart-version", "", "Operator Helm chart version to install on KUBERNETES clusters (default: desired chart version)")
	flags.BoolVarP(&adminClusterOperatorUpdateExecute, "disable-dry-run", "y", false, "Disable dry run mode")
	flags.BoolVarP(&adminClusterOperatorUpdateNoConfirm, "no-confirm", "c", false, "Do not prompt for confirmation")
	flags.IntVarP(&adminClusterOperatorUpdateParallelRuns, "parallel-run", "n", 1, "Number of clusters to update in parallel - must be between 1 and 10")
	flags.IntVarP(&adminClusterOperatorUpdateRefreshDelay, "refresh-delay", "r", 30, "Time in seconds between progress checks - must be between 5 and 120")
	flags.DurationVar(&adminClusterOperatorUpdateTimeout, "timeout", 15*time.Minute, "Maximum time to wait for each cluster update")
}
