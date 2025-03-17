package selfmanaged

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"

	mockCluster "github.com/qovery/qovery-cli/pkg/cluster"
	mockContainerRegistry "github.com/qovery/qovery-cli/pkg/cluster/containerregistry"
	mockCredentials "github.com/qovery/qovery-cli/pkg/cluster/credentials"
	mockOrganization "github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

func TestCreateCluster(t *testing.T) {
	t.Run("Should create a new self managed cluster without creating credentials for AWS (same behavior for SCW & GCP)", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		mockCluster.MockCreateCluster(organization)

		// given
		var clusterService = mockCluster.ClusterServiceMock{
			ResultListClusterRegions: func() (*qovery.ClusterRegionResponseList, error) {
				return &qovery.ClusterRegionResponseList{Results: []qovery.ClusterRegion{{Name: "eu-west-3", CountryCode: "FR", Country: "France", City: "Paris"}}}, nil
			},
		}
		var clusterCredentialsService = mockCredentials.ClusterCredentialsServiceMock{
			ResultListClusterCredentials: func() (*qovery.ClusterCredentialsResponseList, error) {
				return &qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
					{AwsStaticClusterCredentials: &qovery.AwsStaticClusterCredentials{Id: "id-credentials", Name: "AWS credentials"}},
				}}, nil
			},
			ResultAskToCreateCredentials: func() (*qovery.ClusterCredentials, error) {
				// Returns an error if the test asks to create credentials
				return nil, fmt.Errorf("should never ask to create credentials")
			},
		}
		var clusterContainerRegistryService = mockContainerRegistry.ContainerRegistryServiceMock{
			ResultAskToEditClusterContainerRegistry: nil,
		}

		service := NewSelfManagedClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			&clusterService,
			&clusterCredentialsService,
			&clusterContainerRegistryService,
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Select the region where your cluster is installed": "eu-west-3",
				"Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.": "AWS credentials",
			}),
		)

		// when
		var cluster, err = service.Create(organization.Id, qovery.CLOUDVENDORENUM_AWS)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, cluster)
	})
	t.Run("Should create a new self managed cluster without creating credentials for On Premise cluster", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		mockCluster.MockCreateCluster(organization)

		// given
		var clusterService = mockCluster.ClusterServiceMock{
			ResultListClusterRegions: func() (*qovery.ClusterRegionResponseList, error) {
				return nil, fmt.Errorf("should never ask for regions for on premise cluster creation")
			},
		}
		var clusterCredentialsService = mockCredentials.ClusterCredentialsServiceMock{
			ResultListClusterCredentials: func() (*qovery.ClusterCredentialsResponseList, error) {
				return &qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
					{GenericClusterCredentials: &qovery.GenericClusterCredentials{Id: "id-credentials", Name: "AWS credentials"}},
				}}, nil
			},

			ResultAskToCreateCredentials: func() (*qovery.ClusterCredentials, error) {
				// Returns an error if the test asks to create credentials
				return nil, fmt.Errorf("should never ask to create credentials")
			},
		}
		var clusterContainerRegistryService = mockContainerRegistry.ContainerRegistryServiceMock{
			ResultAskToEditClusterContainerRegistry: nil,
		}

		service := NewSelfManagedClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			&clusterService,
			&clusterCredentialsService,
			&clusterContainerRegistryService,
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.": "AWS credentials",
			}),
		)

		// when
		var cluster, err = service.Create(organization.Id, qovery.CLOUDVENDORENUM_ON_PREMISE)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, cluster)
	})
}

func TestConfigureCluster(t *testing.T) {
	t.Run("Should succeed to configure a self managed cluster for a On Premise cluster", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var cluster = mockCluster.CreateTestCluster(mockOrganization.CreateTestOrganization())
		cluster.SetCloudProvider(qovery.CLOUDVENDORENUM_ON_PREMISE)

		// given
		var clusterService = mockCluster.ClusterServiceMock{}
		var clusterCredentialsService = mockCredentials.ClusterCredentialsServiceMock{}
		var clusterContainerRegistryService = mockContainerRegistry.ContainerRegistryServiceMock{
			ResultAskToEditClusterContainerRegistry: nil,
		}

		service := NewSelfManagedClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			&clusterService,
			&clusterCredentialsService,
			&clusterContainerRegistryService,
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var err = service.Configure(cluster)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, cluster)
	})
}

func TestGetInstallationHelmValues(t *testing.T) {
	t.Run("Should get installation helm values cluster id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		organization := mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		MockGetInstallationHelmValues(organization, cluster)

		// given
		service := NewSelfManagedClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			nil,
			nil,
			nil,
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var content, err = service.GetInstallationHelmValues(organization.Id, cluster.Id)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, content)
	})
}

func TestGetBaseHelmValuesContent(t *testing.T) {
	testCases := []struct {
		CloudProviderType qovery.CloudProviderEnum
	}{
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_AWS},
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_SCW},
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_GCP},
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_ON_PREMISE},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("Should get installation helm values cluster cloud provider %s", testCase.CloudProviderType), func(t *testing.T) {
			// given
			service := NewSelfManagedClusterService(
				utils.GetQoveryClient("Fake token type", "Fake token"),
				nil,
				nil,
				nil,
				promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
			)

			// when
			var content, err = service.GetBaseHelmValuesContent(testCase.CloudProviderType)

			// then
			assert.Nil(t, err)
			assert.NotNil(t, content)
		})
	}
}
