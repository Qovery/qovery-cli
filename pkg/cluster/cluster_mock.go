//go:build testing

package cluster

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"net/http"
	"time"
)

var allAdvancedSettingsByClusterId = make(map[string]qovery.ClusterAdvancedSettings)

func CreateTestCluster(organization *qovery.Organization) *qovery.Cluster {
	return qovery.NewCluster(uuid.NewString(), time.Now(), qovery.ReferenceObject{Id: organization.Id}, "TestCluster", "eu-west-3", qovery.CLOUDVENDORENUM_AWS)
}

func MockListClusters(organization *qovery.Organization, clusters []qovery.Cluster) {
	var listClustersResponse = qovery.ClusterResponseList{Results: clusters}
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, listClustersResponse)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockDeployCluster(organization *qovery.Organization, cluster *qovery.Cluster, clusterState *qovery.ClusterStateEnum) {
	var clusterStatus = qovery.ClusterStatus{
		ClusterId: &cluster.Id,
		Status:    clusterState,
	}
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/deploy")
	httpmock.RegisterResponder("POST", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, clusterStatus)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockStopCluster(organization *qovery.Organization, cluster *qovery.Cluster, clusterState *qovery.ClusterStateEnum) {
	var clusterStatus = qovery.ClusterStatus{
		ClusterId: &cluster.Id,
		Status:    clusterState,
	}
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/stop")
	httpmock.RegisterResponder("POST", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, clusterStatus)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockGetClusterStatus(organization *qovery.Organization, cluster *qovery.Cluster, clusterState *qovery.ClusterStateEnum) {
	var clusterStatus = qovery.ClusterStatus{
		ClusterId: &cluster.Id,
		Status:    clusterState,
	}
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/status")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, clusterStatus)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockCreateCluster(organization *qovery.Organization) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster")
	httpmock.RegisterResponder("POST", url,
		func(req *http.Request) (*http.Response, error) {
			// Decode & store the cluster request
			var clusterRequest qovery.ClusterRequest
			if err := json.NewDecoder(req.Body).Decode(&clusterRequest); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			var clusterResponse = qovery.NewClusterWithDefaults()
			clusterResponse.Id = uuid.NewString()
			clusterResponse.CreatedAt = time.Now()
			clusterResponse.UpdatedAt = nil
			clusterResponse.Organization = qovery.ReferenceObject{Id: organization.Id}
			clusterResponse.Region = clusterRequest.Region
			clusterResponse.CloudProvider = clusterRequest.CloudProvider
			clusterResponse.Kubernetes = clusterRequest.Kubernetes
			resp, err := httpmock.NewJsonResponse(200, clusterResponse)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)
}

func MockGetClusterAdvancedSettings(organization *qovery.Organization, cluster *qovery.Cluster, advancedSettings *qovery.ClusterAdvancedSettings) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/advancedSettings")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, advancedSettings)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockEditClusterAdvancedSettings(organization *qovery.Organization, cluster *qovery.Cluster) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/advancedSettings")
	httpmock.RegisterResponder("PUT", url,
		func(req *http.Request) (*http.Response, error) {
			var advancedSettings qovery.ClusterAdvancedSettings
			if err := json.NewDecoder(req.Body).Decode(&advancedSettings); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			allAdvancedSettingsByClusterId[cluster.Id] = advancedSettings

			resp, err := httpmock.NewJsonResponse(200, advancedSettings)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockListCloudProviderRegion(cloudProviderType qovery.CloudProviderEnum, regions []qovery.ClusterRegion) {
	var cloudProviderTypeApi string
	switch cloudProviderType {
	case qovery.CLOUDPROVIDERENUM_AWS:
		cloudProviderTypeApi = "aws"
	case qovery.CLOUDPROVIDERENUM_SCW:
		cloudProviderTypeApi = "scaleway"
	case qovery.CLOUDPROVIDERENUM_GCP:
		cloudProviderTypeApi = "gcp"
	case qovery.CLOUDPROVIDERENUM_ON_PREMISE:
		cloudProviderTypeApi = "onPremise"
	}
	var url = fmt.Sprintf("https://api.qovery.com/%s/region", cloudProviderTypeApi)
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, qovery.ClusterRegionResponseList{Results: regions})
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

// TODO (mzo) set all ResultXXX to a func() error to be coherent with others
type ClusterServiceMock struct {
	ResultDeployCluster         error
	ResultStopCluster           error
	ResultListClusters          func() (*qovery.ClusterResponseList, error)
	ResultListClusterRegions    func() (*qovery.ClusterRegionResponseList, error)
	ResultAskToEditStorageClass error
}

func (mock *ClusterServiceMock) DeployCluster(organizationName string, clusterName string, watchFlag bool) error {
	return mock.ResultDeployCluster
}
func (mock *ClusterServiceMock) StopCluster(organizationName string, clusterName string, watchFlag bool) error {
	return mock.ResultStopCluster
}
func (mock *ClusterServiceMock) ListClusters(organizationId string) (*qovery.ClusterResponseList, error) {
	return mock.ResultListClusters()
}
func (mock *ClusterServiceMock) ListClusterRegions(cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterRegionResponseList, error) {
	return mock.ResultListClusterRegions()
}
func (mock *ClusterServiceMock) AskToEditStorageClass(cluster *qovery.Cluster) error {
	return mock.ResultAskToEditStorageClass
}
