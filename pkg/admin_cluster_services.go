package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

//
// DTO

type ListOfClustersEligibleToUpdate struct {
	Results []ClusterDetails
}
type ClusterDetails struct {
	OrganizationId        string `json:"organization_id"`
	OrganizationName      string `json:"organization_name"`
	OrganizationPlan      string `json:"organization_plan"`
	ClusterId             string `json:"cluster_id"`
	ClusterName           string `json:"cluster_name"`
	ClusterType           string `json:"cluster_type"`
	ClusterCreatedAt      string `json:"cluster_created_at"`
	ClusterLastDeployedAt string `json:"cluster_last_deployed_at"`
	ClusterK8sVersion     string `json:"cluster_k8s_version"`
	Mode                  string `json:"mode"`
	IsProduction          bool   `json:"is_production"`
	CurrentStatus         string `json:"current_status"`
	HasKarpenter          bool   `json:"has_karpenter"`
	HasPendingUpdate      bool   `json:"has_pending_update"`
}

// PrintClustersTable global method to output clusters table
func PrintClustersTable(clusters []ClusterDetails) error {
	var data [][]string

	utils.Println("")
	for _, cluster := range clusters {
		data = append(data, []string{
			cluster.OrganizationId,
			cluster.OrganizationName,
			cluster.OrganizationPlan,
			cluster.ClusterId,
			cluster.ClusterName,
			cluster.ClusterType,
			cluster.ClusterK8sVersion,
			cluster.Mode,
			strconv.FormatBool(cluster.IsProduction),
			cluster.CurrentStatus,
			strconv.FormatBool(cluster.HasKarpenter),
			cluster.ClusterCreatedAt,
			cluster.ClusterLastDeployedAt,
			strconv.FormatBool(cluster.HasPendingUpdate),
		})
	}

	err := utils.PrintTable([]string{
		"OrganizationId",
		"OrganizationName",
		"OrganizationPlan",
		"ClusterId",
		"ClusterName",
		"ClusterType",
		"ClusterK8sVersion",
		"Mode",
		"IsProduction",
		"CurrentStatus",
		"HasKarpenter",
		"ClusterCreatedAt",
		"ClusterLastDeployedAt",
		"HasPendingUpdate",
	}, data)
	if err != nil {
		return fmt.Errorf("cannot print clusters %s", err)
	}
	return nil
}

// Service to list clusters
var allowedFilterProperties = map[string]bool{
	"OrganizationId":    true,
	"OrganizationName":  true,
	"OrganizationPlan":  true,
	"ClusterId":         true,
	"ClusterName":       true,
	"ClusterType":       true,
	"ClusterK8sVersion": true,
	"CurrentStatus":     true,
	"Mode":              true,
	"IsProduction":      true,
	"HasKarpenter":      true,
	"HasPendingUpdate":  true,
}

type AdminClusterListService interface {
	SelectClusters() ([]ClusterDetails, error)
}

type AdminClusterListServiceImpl struct {
	// Filters based on ClusterDetails struct fields (reflection is used to filter fields)
	Filters map[string]string
}

func NewAdminClusterListServiceImpl(filters map[string]string) (*AdminClusterListServiceImpl, error) {
	if len(filters) > 0 {
		for key := range filters {
			_, keyIsPresent := allowedFilterProperties[key]
			if !keyIsPresent {
				keys := make([]string, len(allowedFilterProperties))
				i := 0
				for k := range allowedFilterProperties {
					keys[i] = k
					i++
				}
				return nil, fmt.Errorf("Filter property '%s' not available: valid values are: "+strings.Join(keys, ", "), key)
			}
		}
	}

	return &AdminClusterListServiceImpl{
		Filters: filters,
	}, nil
}

func (service AdminClusterListServiceImpl) SelectClusters() ([]ClusterDetails, error) {
	clustersFetched, err := service.fetchClustersEligibleToUpdate()
	if err != nil {
		return nil, err
	}
	clusters := service.filterByPredicates(clustersFetched, service.Filters)
	return clusters, nil
}

func (service AdminClusterListServiceImpl) fetchClustersEligibleToUpdate() ([]ClusterDetails, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, utils.GetAdminUrl()+"/listClustersEligibleToUpdate", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("cannot fetch clusters (status_code=%d)", res.StatusCode)
	}

	list := ListOfClustersEligibleToUpdate{}
	err = json.NewDecoder(res.Body).Decode(&list)
	if err != nil {
		return nil, err
	}

	return list.Results, nil
}

func (service AdminClusterListServiceImpl) filterByPredicates(clusters []ClusterDetails, filters map[string]string) []ClusterDetails {
	var filteredClusters []ClusterDetails
	for _, cluster := range clusters {
		matchAllFilters := true
		for filterProperty, filterValue := range filters {
			filterValuesSet := service.filterValueToHashSet(filterValue)
			clusterProperty := reflect.Indirect(reflect.ValueOf(cluster)).FieldByName(filterProperty)

			// hack for IsProduction field (boolean needs to be converted to string)
			if filterProperty == "IsProduction" || filterProperty == "HasKarpenter" || filterProperty == "HasPendingUpdate" {
				boolToString := strconv.FormatBool(clusterProperty.Bool())
				if _, ok := filterValuesSet[boolToString]; !ok {
					matchAllFilters = false
				}
			} else {
				if _, ok := filterValuesSet[clusterProperty.String()]; !ok {
					matchAllFilters = false
				}
			}

			if !matchAllFilters {
				break
			}
		}

		if matchAllFilters {
			filteredClusters = append(filteredClusters, cluster)
		}
	}
	return filteredClusters
}

// filterValueToHashSet Actually it's a hashmap but golang has no hashset
func (service AdminClusterListServiceImpl) filterValueToHashSet(filterValue string) map[string]bool {
	splitFilterValue := strings.Split(filterValue, ",")
	hashmap := make(map[string]bool, len(splitFilterValue))

	for _, value := range splitFilterValue {
		hashmap[value] = true
	}

	return hashmap
}

//
// Service to deploy clusters

type ClusterBatchDeployResult struct {
	// ProcessedClusters clusters that have been processed, non matter the final state created
	ProcessedClusters []ClusterDetails
	// PendingClusters clusters in the pending queue (their state were not in ready state)
	PendingClusters []ClusterDetails
}

type AdminClusterBatchDeployService interface {
	Deploy(clusters []ClusterDetails) (*ClusterBatchDeployResult, error)
	PrintParameters()
}

type AdminClusterBatchDeployServiceImpl struct {
	// DryRunDisabled disable dry run
	DryRunDisabled bool
	// ParallelRun the number of parallel requests to be processed
	ParallelRun int
	// RefreshDelay the delay to fetch cluster status in process
	RefreshDelay int
	// CompleteBatchBeforeContinue to block on N parallel runs to be processed: true = 'batch' mode / false = 'on-the-fly' mode
	CompleteBatchBeforeContinue bool
	// UpgradeClusterNewK8sVersion indicates next version to trigger a cluster upgrade
	UpgradeClusterNewK8sVersion *string
	// UpgradeMode indicates if the cluster needs to be upgraded
	UpgradeMode bool
	// NoConfirm do not prompt for any confirmation
	NoConfirm bool
}

func NewAdminClusterBatchDeployServiceImpl(
	dryRun bool,
	parallelRun int,
	refreshDelay int,
	executionMode string,
	newK8sversionStr string,
	noConfirm bool,
) (*AdminClusterBatchDeployServiceImpl, error) {
	// set at least 1 parallel run
	if parallelRun < 1 {
		parallelRun = 1
	}
	if parallelRun > 20 && !noConfirm {
		utils.Println("")
		utils.Println(fmt.Sprintf("Please increase the cluster engine autoscaler to %d, then type 'yes' to continue", parallelRun))
		validated := utils.Validate("autoscaler-increase")
		if !validated {
			utils.Println("Exiting")
			return nil, fmt.Errorf("exit on autoscaler validation failed")
		}
		utils.Println("")
	}

	var newK8sVersion *string = nil
	upgradeMode := false
	if newK8sversionStr != "" {
		newK8sVersion = &newK8sversionStr
		upgradeMode = true
	}

	completeBatchBeforeContinue := executionMode != "on-the-fly" || upgradeMode

	return &AdminClusterBatchDeployServiceImpl{
		DryRunDisabled:              dryRun,
		ParallelRun:                 parallelRun,
		RefreshDelay:                refreshDelay,
		CompleteBatchBeforeContinue: completeBatchBeforeContinue,
		UpgradeClusterNewK8sVersion: newK8sVersion,
		UpgradeMode:                 upgradeMode,
	}, nil
}

func (service AdminClusterBatchDeployServiceImpl) PrintParameters() {
	utils.Println("-------------------------------------------")
	utils.Println(fmt.Sprintf("- DryRunDisabled: %t", service.DryRunDisabled))
	utils.Println(fmt.Sprintf("- ParallelRun: %d", service.ParallelRun))
	utils.Println(fmt.Sprintf("- RefreshDelay: %d seconds", service.RefreshDelay))
	utils.Println(fmt.Sprintf("- BatchMode: %t", service.CompleteBatchBeforeContinue))
	if service.UpgradeMode {
		utils.Println(fmt.Sprintf("- UpgradeMode: true (NewK8sVersion = %s)", *service.UpgradeClusterNewK8sVersion))
	} else {
		utils.Println("- UpgradeMode: false")
	}
	utils.Println("-------------------------------------------")
}

func getQoveryClient() (*qovery.APIClient, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}
	return utils.GetQoveryClient(tokenType, token), nil
}

func (service AdminClusterBatchDeployServiceImpl) Deploy(clusters []ClusterDetails) (*ClusterBatchDeployResult, error) {
	if !service.DryRunDisabled {
		utils.Println("dry-run-disabled is false: trigger cluster deployment dry-run mode (no changes will be made)")
	}

	// store final state of clusters in a hashmap
	var processedClusters []ClusterDetails
	// store the current status for each cluster deployed, to be able to execute next parallel runs
	currentDeployingClustersByClusterId := make(map[string]ClusterDetails)
	// clusters having a non-terminal state when trying to deploy them
	var pendingClusters []ClusterDetails

	indexCurrentClusterToDeploy := -1
	for {
		// fetch Qovery client
		qoveryClient, err := getQoveryClient()
		if err != nil {
			return nil, err
		}

		// boolean to wait for current batch to continue, according to 'execution-mode' command flag
		waitToTriggerCluster := false
		if service.CompleteBatchBeforeContinue && indexCurrentClusterToDeploy != -1 {
			if len(currentDeployingClustersByClusterId) > 0 {
				waitToTriggerCluster = true
			} else {
				utils.Println(fmt.Sprintf("Do you want to continue next batch of %d deployments ?", service.ParallelRun))
				validated := utils.Validate("deploy")
				if !validated {
					utils.Println("Exiting")
					return nil, fmt.Errorf("user stopped the command after batch terminated")
				}
			}
		}

		// if enough space to start a new cluster deployment
		if !waitToTriggerCluster && len(currentDeployingClustersByClusterId) < service.ParallelRun && indexCurrentClusterToDeploy < len(clusters)-1 {
			// fill the hashmap according to parallel runs
			for i := len(currentDeployingClustersByClusterId); i < service.ParallelRun; i++ {
				indexCurrentClusterToDeploy += 1

				// check status in case a deployment has occurred in the meantime
				cluster := clusters[indexCurrentClusterToDeploy]

				clusterStatus, response, err := RetryQoveryClientApiRequestOnUnauthorized(func(needToRefetchClient bool) (*qovery.ClusterStatus, *http.Response, error) {
					if needToRefetchClient {
						client, errQoveryClient := getQoveryClient()
						if errQoveryClient != nil {
							return nil, nil, errQoveryClient
						}
						qoveryClient = client
					}
					return qoveryClient.ClustersAPI.GetClusterStatus(context.Background(), cluster.OrganizationId, cluster.ClusterId).Execute()
				})
				if response == nil || response.StatusCode > 200 || err != nil {
					return nil, err
				}

				// Trigger a deployment only when the target status is in terminal state
				if utils.IsTerminalClusterState(*clusterStatus.Status) {
					utils.Println(fmt.Sprintf("[Organization '%s' - Cluster '%s'] - Starting deployment - https://console.qovery.com/organization/%s/cluster/%s/logs", cluster.OrganizationName, cluster.ClusterName, cluster.OrganizationId, cluster.ClusterId))
					var err error
					if service.UpgradeClusterNewK8sVersion != nil {
						err = service.upgradeCluster(cluster.ClusterId, *service.UpgradeClusterNewK8sVersion, service.DryRunDisabled)
					} else {
						err = service.deployCluster(cluster.ClusterId, service.DryRunDisabled)
					}
					if err != nil {
						utils.Println(fmt.Sprintf("[Organization '%s' - Cluster '%s'] - Error on deploy: %s ", cluster.OrganizationName, cluster.ClusterName, err))
					}
					cluster.CurrentStatus = "DEPLOYING"
					currentDeployingClustersByClusterId[cluster.ClusterId] = cluster
				} else {
					status := fmt.Sprintf("%v", *clusterStatus.Status) // only solution to get the underlying enum's string value
					utils.Println(fmt.Sprintf("[Organization '%s' - Cluster '%s'] - Cluster's state is '%s' (not a terminal state), sending it to waiting queue to be processed later", cluster.OrganizationName, cluster.ClusterName, status))
					pendingClusters = append(pendingClusters, cluster)
				}

				// if last cluster has been reached, break
				if indexCurrentClusterToDeploy == len(clusters)-1 {
					break
				}
			}
		}

		// sleep some time before fetching statuses
		utils.Println(fmt.Sprintf("Checking clusters' status in %d seconds", service.RefreshDelay))
		time.Sleep(time.Duration(service.RefreshDelay) * time.Second)

		// wait for clusters statuses
		var clustersToRemoveFromMap []string
		for clusterId, cluster := range currentDeployingClustersByClusterId {
			clusterStatus, response, err := RetryQoveryClientApiRequestOnUnauthorized(func(needToRefetchClient bool) (*qovery.ClusterStatus, *http.Response, error) {
				if needToRefetchClient {
					client, errQoveryClient := getQoveryClient()
					if errQoveryClient != nil {
						return nil, nil, errQoveryClient
					}
					qoveryClient = client
				}
				return qoveryClient.ClustersAPI.GetClusterStatus(context.Background(), cluster.OrganizationId, cluster.ClusterId).Execute()
			})
			if response == nil || response.StatusCode > 200 || err != nil {
				return nil, err
			}

			// set cluster status
			status := fmt.Sprintf("%v", *clusterStatus.Status) // only solution to get the underlying enum's string value
			cluster.CurrentStatus = status
			// Mark the deployment as finished only if terminal state OR status is "INTERNAL_ERROR" (specific case)
			if utils.IsTerminalClusterState(*clusterStatus.Status) || cluster.CurrentStatus == "INTERNAL_ERROR" {
				utils.Println(fmt.Sprintf("[Organization '%s' - Cluster '%s'] - Cluster deployed with '%s' status ", cluster.OrganizationName, cluster.ClusterName, *clusterStatus.Status))

				processedClusters = append(processedClusters, cluster)
				clustersToRemoveFromMap = append(clustersToRemoveFromMap, clusterId)
			}
		}

		// remove deployed clusters
		for _, clusterId := range clustersToRemoveFromMap {
			delete(currentDeployingClustersByClusterId, clusterId)
		}

		// check if every cluster has been deployed
		if len(currentDeployingClustersByClusterId) == 0 && indexCurrentClusterToDeploy == len(clusters)-1 {
			break
		}
	}

	utils.Println("No more deployment to process")

	return &ClusterBatchDeployResult{
		ProcessedClusters: processedClusters,
		PendingClusters:   pendingClusters,
	}, nil
}

func (service AdminClusterBatchDeployServiceImpl) deployCluster(clusterId string, dryRunDisabled bool) error {
	adminUrl := utils.GetAdminUrl()
	response := execAdminRequest(adminUrl+"/cluster/deploy/"+clusterId, http.MethodPost, dryRunDisabled, map[string]string{})
	if response.StatusCode == 401 {
		DoRequestUserToAuthenticate(false)
		response = execAdminRequest(adminUrl+"/cluster/deploy/"+clusterId, http.MethodPost, dryRunDisabled, map[string]string{})
	}
	if response.StatusCode != 200 {
		result, _ := io.ReadAll(response.Body)
		return fmt.Errorf("could not deploy cluster : %s. %s", response.Status, string(result))
	}
	return nil
}

func (service AdminClusterBatchDeployServiceImpl) upgradeCluster(clusterId string, targetVersion string, dryRunDisabled bool) error {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	adminUrl := utils.GetAdminUrl()

	body := bytes.NewBuffer([]byte(fmt.Sprintf("{ \"metadata\": { \"dry_run_deploy\": \"%s\", \"target_version\": \"%s\" } }", strconv.FormatBool(!dryRunDisabled), targetVersion)))
	request, err := http.NewRequest(http.MethodPost, adminUrl+"/cluster/update/"+clusterId, body)
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode == 401 {
		DoRequestUserToAuthenticate(false)
		request, err = http.NewRequest(http.MethodPost, adminUrl+"/cluster/update/"+clusterId, body)
		if err != nil {
			return err
		}
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
	}

	if response.StatusCode != 200 {
		result, _ := io.ReadAll(response.Body)
		return fmt.Errorf("could not deploy cluster : %s. %s", response.Status, string(result))
	}
	return nil
}
