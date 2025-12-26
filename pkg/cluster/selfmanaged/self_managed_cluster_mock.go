package selfmanaged

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"net/http"
	"time"
)

type SelfManagedClusterServiceMock struct {
	ResultCreate                    func(organizationID string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error)
	ResultConfigure                 func() error
	ResultGetInstallationHelmValues func() (*string, error)
	ResultGetBaseHelmValuesContent  func(kubernetesType qovery.CloudProviderEnum) (*string, error)
}

func (mock *SelfManagedClusterServiceMock) Create(organizationID string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
	return mock.ResultCreate(organizationID, cloudVendor)
}
func (mock *SelfManagedClusterServiceMock) Configure(cluster *qovery.Cluster) error {
	return mock.ResultConfigure()
}
func (mock *SelfManagedClusterServiceMock) GetInstallationHelmValues(organizationId string, clusterId string) (*string, error) {
	return mock.ResultGetInstallationHelmValues()
}
func (mock *SelfManagedClusterServiceMock) GetBaseHelmValuesContent(kubernetesType qovery.CloudProviderEnum) (*string, error) {
	return mock.ResultGetBaseHelmValuesContent(kubernetesType)
}

func CreateSelfManagedTestCluster(organization *qovery.Organization, cloudVendor qovery.CloudVendorEnum) *qovery.Cluster {
	cluster := qovery.NewCluster(uuid.NewString(), time.Now(), qovery.ReferenceObject{Id: organization.Id}, "TestCluster", "eu-west-3", cloudVendor)
	cluster.SetKubernetes(qovery.KUBERNETESENUM_SELF_MANAGED)
	return cluster
}

func MockGetInstallationHelmValues(organization *qovery.Organization, cluster *qovery.Cluster) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/cluster/", cluster.Id, "/installationHelmValues")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, "<mock-content-installation-helm-values>")
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}
