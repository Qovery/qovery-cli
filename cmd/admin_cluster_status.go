package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/appscode/go-querystring/query"
	"github.com/gorilla/websocket"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminClusterStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Get cluster status",
		Run: func(cmd *cobra.Command, args []string) {
			printClusterStatus()
		},
	}
)

func init() {
	adminClusterStatusCmd.Flags().StringVar(&organizationId, "organization-id", "", "The cluster's organization ")
	adminClusterStatusCmd.Flags().StringVar(&clusterId, "cluster-id", "", "The cluster id to target")
	adminClusterCmd.AddCommand(adminClusterStatusCmd)
}

func printClusterStatus() {
	status, err := readClusterStatus(ClusterStatusRequest{
		ClusterID:      utils.Id(clusterId),
		OrganizationID: utils.Id(organizationId),
	})
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	renderClusterStatus(status)
}

func readClusterStatus(req ClusterStatusRequest) (*ClusterStatusDto, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}
	websocketUrl := utils.WebsocketUrl()

	wsURL, err := url.Parse(fmt.Sprintf("%s/cluster/status", websocketUrl))
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile("%5B([0-9]+)%5D=")
	wsURL.RawQuery = pattern.ReplaceAllString(command.Encode(), "[${1}]=")

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	headers := http.Header{"Authorization": {utils.GetAuthorizationHeaderValue(tokenType, token)}}
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = wsConn.Close()
	}()

	msgType, payload, err := wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}

	switch msgType {
	case websocket.TextMessage:
		var data ClusterStatusDto
		err = json.Unmarshal(payload, &data)
		if err != nil {
			return nil, err
		}
		return &data, nil
	default:
		return nil, errors.New("received invalid message while fetching cluster status: " + string(rune(msgType)) + " " + string(payload))
	}
}

func renderClusterStatus(clusterStatus *ClusterStatusDto) {
	// Write the header
	fmt.Printf("%-71s %-20s %-20s %-20s %-20s %-20s\n",
		"",
		pterm.Bold.Sprintf("%s", "RAM Alloc"),
		pterm.Bold.Sprintf("%s", "RAM Usage"),
		pterm.Bold.Sprintf("%s", "CPU Alloc"),
		pterm.Bold.Sprintf("%s", "CPU Usage"),
		pterm.Bold.Sprintf("%s", "Disk Usage"),
	)
	fmt.Println("")

	// Sort nodes by name for consistent output
	sortedNodes := make([]ClusterNodeDto, len(clusterStatus.Nodes))
	copy(sortedNodes, clusterStatus.Nodes)
	sort.Slice(sortedNodes, func(i, j int) bool {
		return sortedNodes[i].Name < sortedNodes[j].Name
	})

	// Process each node
	for i, node := range sortedNodes {
		isLastNode := i == len(sortedNodes)-1

		// Format node metrics
		ramAlloc := fmt.Sprintf("%dMi", node.ResourcesAllocated.MemoryMib)

		var ramUsage string
		if node.MetricsUsage.MemoryMibRssUsage != nil && node.MetricsUsage.MemoryPercentRssUsage != nil {
			ramUsage = fmt.Sprintf("%dMi(%d%%)", *node.MetricsUsage.MemoryMibRssUsage, *node.MetricsUsage.MemoryPercentRssUsage)
		} else {
			ramUsage = "--(--%)"
		}

		cpuAlloc := fmt.Sprintf("%dm", node.ResourcesAllocated.CpuMilli)

		var cpuUsage string
		if node.MetricsUsage.CpuMilliUsage != nil && node.MetricsUsage.CpuPercentUsage != nil {
			cpuUsage = fmt.Sprintf("%dm(%d%%)", *node.MetricsUsage.CpuMilliUsage, *node.MetricsUsage.CpuPercentUsage)
		} else {
			cpuUsage = "--(--%)"
		}

		var diskUsage string
		if node.MetricsUsage.DiskMibUsage != nil && node.MetricsUsage.DiskPercentUsage != nil {
			diskUsage = fmt.Sprintf("%dMi(%d%%)", *node.MetricsUsage.DiskMibUsage, *node.MetricsUsage.DiskPercentUsage)
		} else {
			diskUsage = "--(--%)"
		}

		// Print node information
		fmt.Printf("%-79s %-20s %-20s %-20s %-20s %-20s\n",
			pterm.Bold.Sprintf("%s", node.Name),
			pterm.Bold.Sprintf("%s", ramAlloc),
			pterm.Bold.Sprintf("%s", ramUsage),
			pterm.Bold.Sprintf("%s", cpuAlloc),
			pterm.Bold.Sprintf("%s", cpuUsage),
			pterm.Bold.Sprintf("%s", diskUsage))

		// Sort pods by name for consistent output
		sortedPods := make([]NodePodInfoDto, len(node.Pods))
		copy(sortedPods, node.Pods)
		sort.Slice(sortedPods, func(i, j int) bool {
			return sortedPods[i].Name < sortedPods[j].Name
		})

		// Process each pod in the node
		for j, pod := range sortedPods {
			isLastPod := j == len(sortedPods)-1

			// Determine the appropriate pod prefix symbol
			var podPrefix string
			if isLastPod {
				podPrefix = "└─ "
			} else {
				podPrefix = "├─ "
			}

			// Format pod metrics
			var podRamAlloc string
			if pod.MemoryMibRequest != nil {
				podRamAlloc = fmt.Sprintf("%dMi", *pod.MemoryMibRequest)
			} else {
				podRamAlloc = "--"
			}

			var podRamUsage string
			if pod.MetricsUsage.MemoryMibRssUsage != nil && pod.MetricsUsage.MemoryPercentRssUsage != nil {
				podRamUsage = fmt.Sprintf("%dMi(%d%%)", *pod.MetricsUsage.MemoryMibRssUsage, *pod.MetricsUsage.MemoryPercentRssUsage)
			} else {
				podRamUsage = "--(--%)"
			}

			var podCpuAlloc string
			if pod.CpuMilliRequest != nil {
				podCpuAlloc = fmt.Sprintf("%dm", *pod.CpuMilliRequest)
			} else {
				podCpuAlloc = "--"
			}

			var podCpuUsage string
			if pod.MetricsUsage.CpuMilliUsage != nil && pod.MetricsUsage.CpuPercentUsage != nil {
				podCpuUsage = fmt.Sprintf("%dm(%d%%)", *pod.MetricsUsage.CpuMilliUsage, *pod.MetricsUsage.CpuPercentUsage)
			} else {
				podCpuUsage = "--(--%)"
			}

			var podDiskUsage string
			if pod.MetricsUsage.DiskMibUsage != nil && pod.MetricsUsage.DiskPercentUsage != nil {
				podDiskUsage = fmt.Sprintf("%dMi(%d%%)", *pod.MetricsUsage.DiskMibUsage, *pod.MetricsUsage.DiskPercentUsage)
			} else {
				podDiskUsage = "--(--%)"
			}

			var podName string
			if len(pod.ErrorContainerStatuses) > 0 {
				podName = fmt.Sprintf("%-77s", pterm.Red(pod.Name))
			} else {
				podName = fmt.Sprintf("%-68s", pod.Name)
			}

			// Print pod information
			fmt.Printf("%s%s %-12s %-12s %-12s %-12s %-12s\n",
				podPrefix,
				podName,
				podRamAlloc,
				podRamUsage,
				podCpuAlloc,
				podCpuUsage,
				podDiskUsage,
			)
		}

		// Add blank line between nodes
		if !isLastNode {
			fmt.Printf("\n")
		}
	}
}

type ClusterStatusRequest struct {
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
}

type ClusterStatusDto struct {
	ComputedStatus ClusterComputedStatusDto `json:"computed_status"`
	Nodes          []ClusterNodeDto         `json:"nodes"`
	Pvcs           []PvcInfoDto             `json:"pvcs"`
}

type ClusterComputedStatusDto struct {
	GlobalStatus              ClusterStatusGlobalStatus      `json:"global_status"`
	QoveryComponentsInFailure []QoveryComponentInFailure     `json:"qovery_components_in_failure"`
	NodeWarnings              map[string][]QoveryNodeFailure `json:"node_warnings"`
	IsMaxNodesSizeReached     bool                           `json:"is_max_nodes_size_reached"`
	KubeVersionStatus         QoveryClusterKubeVersionStatus `json:"kube_version_status"`
}

type ClusterStatusGlobalStatus string

const (
	ClusterStatusGlobalStatusRunning ClusterStatusGlobalStatus = "RUNNING"
	ClusterStatusGlobalStatusWarning ClusterStatusGlobalStatus = "WARNING"
	ClusterStatusGlobalStatusError   ClusterStatusGlobalStatus = "ERROR"
)

type QoveryComponentInFailure struct {
	Type          string                              `json:"type"`
	ComponentName string                              `json:"component_name"`
	PodName       string                              `json:"pod_name,omitempty"`
	ContainerName string                              `json:"container_name,omitempty"`
	Level         QoveryComponentContainerStatusLevel `json:"level,omitempty"`
	Reason        *string                             `json:"reason,omitempty"`
	Message       *string                             `json:"message,omitempty"`
}

type PodInErrorValue struct {
	ComponentName string                              `json:"component_name"`
	PodName       string                              `json:"pod_name"`
	ContainerName string                              `json:"container_name"`
	Level         QoveryComponentContainerStatusLevel `json:"level"`
	Reason        *string                             `json:"reason"`
	Message       *string                             `json:"message"`
	Type          string                              `json:"type"`
}

type MissingComponentValue struct {
	ComponentName string `json:"component_name"`
	Type          string `json:"type"`
}

type QoveryComponentContainerStatusIssue struct {
	Level   QoveryComponentContainerStatusLevel `json:"level"`
	Reason  *string                             `json:"reason"`
	Message *string                             `json:"message"`
}

type QoveryNodeFailure struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type QoveryComponentContainerStatusLevel string

const (
	QoveryComponentContainerStatusLevelError   QoveryComponentContainerStatusLevel = "ERROR"
	QoveryComponentContainerStatusLevelWarning QoveryComponentContainerStatusLevel = "WARNING"
)

type QoveryClusterKubeVersionStatus struct {
	Type                string `json:"type"`
	KubeVersion         string `json:"kube_version,omitempty"`
	ExpectedKubeVersion string `json:"expected_kube_version,omitempty"`
}

type KubeVersionStatusOkValue struct {
	KubeVersion string `json:"kube_version"`
	Type        string `json:"type"`
}

type KubeVersionStatusDriftValue struct {
	KubeVersion         string `json:"kube_version"`
	ExpectedKubeVersion string `json:"expected_kube_version"`
	Type                string `json:"type"`
}

type KubeVersionStatusUnknownValue struct {
	Type string `json:"type"`
}

type ClusterNodeDto struct {
	CreatedAt            *uint64                  `json:"created_at"`
	Name                 string                   `json:"name"`
	Architecture         string                   `json:"architecture"`
	InstanceType         *string                  `json:"instance_type"`
	KernelVersion        string                   `json:"kernel_version"`
	KubeletVersion       string                   `json:"kubelet_version"`
	OperatingSystem      string                   `json:"operating_system"`
	OsImage              string                   `json:"os_image"`
	Unschedulable        bool                     `json:"unschedulable"`
	ResourcesAllocatable NodeResourceDto          `json:"resources_allocatable"`
	ResourcesAllocated   NodeResourceAllocatedDto `json:"resources_allocated"`
	Taints               []NodeTaintDto           `json:"taints"`
	Conditions           []NodeConditionDto       `json:"conditions"`
	Labels               map[string]string        `json:"labels"`
	Annotations          map[string]string        `json:"annotations"`
	Addresses            []NodeAddressDto         `json:"addresses"`
	Pods                 []NodePodInfoDto         `json:"pods"`
	MetricsUsage         MetricsUsageDto          `json:"metrics_usage"`
}

type NodeTaintDto struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

type NodeConditionDto struct {
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	LastHeartbeatTime  *uint64 `json:"last_heartbeat_time"`
	LastTransitionTime *uint64 `json:"last_transition_time"`
	Reason             string  `json:"reason"`
	Message            string  `json:"message"`
}

type NodeResourceDto struct {
	CpuMilli            uint64 `json:"cpu_milli"`
	MemoryMib           uint64 `json:"memory_mib"`
	EphemeralStorageMib uint64 `json:"ephemeral_storage_mib"`
	Pods                uint64 `json:"pods"`
}

type NodeResourceAllocatedDto struct {
	MemoryMib uint32 `json:"memory_mib"`
	CpuMilli  uint32 `json:"cpu_milli"`
}

type NodePodInfoDto struct {
	CreatedAt              *uint64                  `json:"created_at"`
	Name                   string                   `json:"name"`
	Namespace              string                   `json:"namespace"`
	ErrorContainerStatuses []NodePodErrorStatusDto  `json:"error_container_statuses"`
	QoveryServiceInfo      *PodQoveryServiceInfoDto `json:"qovery_service_info"`
	CpuMilliRequest        *uint32                  `json:"cpu_milli_request"`
	CpuMilliLimit          *uint32                  `json:"cpu_milli_limit"`
	MemoryMibRequest       *uint32                  `json:"memory_mib_request"`
	MemoryMibLimit         *uint32                  `json:"memory_mib_limit"`
	MetricsUsage           MetricsUsageDto          `json:"metrics_usage"`
	ImagesVersion          map[string]string        `json:"images_version"`
	RestartCount           uint32                   `json:"restart_count"`
}

type NodePodErrorStatusDto struct {
	ContainerName string  `json:"container_name"`
	Reason        *string `json:"reason"`
	Message       *string `json:"message"`
}

type PodQoveryServiceInfoDto struct {
	ProjectId       string `json:"project_id"`
	ProjectName     string `json:"project_name"`
	EnvironmentId   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`
	ServiceId       string `json:"service_id"`
	ServiceName     string `json:"service_name"`
}

type MetricsUsageDto struct {
	CpuMilliUsage                *uint32 `json:"cpu_milli_usage"`
	CpuPercentUsage              *uint32 `json:"cpu_percent_usage"`
	MemoryMibRssUsage            *uint32 `json:"memory_mib_rss_usage"`
	MemoryPercentRssUsage        *uint32 `json:"memory_percent_rss_usage"`
	MemoryMibWorkingSetUsage     *uint32 `json:"memory_mib_working_set_usage"`
	MemoryPercentWorkingSetUsage *uint32 `json:"memory_percent_working_set_usage"`
	DiskMibUsage                 *uint32 `json:"disk_mib_usage"`
	DiskPercentUsage             *uint32 `json:"disk_percent_usage"`
}

type NodeAddressDto struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type PvcInfoDto struct {
	Name              string                   `json:"name"`
	Namespace         string                   `json:"namespace"`
	PodName           string                   `json:"pod_name"`
	DiskMibUsage      uint32                   `json:"disk_mib_usage"`
	DiskPercentUsage  uint32                   `json:"disk_percent_usage"`
	DiskMibCapacity   uint32                   `json:"disk_mib_capacity"`
	QoveryServiceInfo *PodQoveryServiceInfoDto `json:"qovery_service_info"`
}
