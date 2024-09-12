package cluster

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"

	mockOrganization "github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

func TestListClusters(t *testing.T) {
	t.Run("Should return empty list if no cluster found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{})

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var clusters, err = service.ListClusters(organization.Id)

		// then
		assert.Nil(t, err)
		assert.Equal(t, 0, len(clusters.GetResults()))
	})
	t.Run("Should list clusters linked to organization selected", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{*cluster})
		deployingState := qovery.CLUSTERSTATEENUM_DEPLOYING
		MockDeployCluster(organization, cluster, &deployingState)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var clusters, err = service.ListClusters(organization.Id)

		// then
		assert.Nil(t, err)
		assert.Equal(t, 1, len(clusters.GetResults()))
		assert.Equal(t, cluster.Id, clusters.GetResults()[0].Id)
	})
}

func TestDeployManagedCluster(t *testing.T) {
	t.Run("Should deploy cluster without waiting for final status", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{*cluster})
		deployingState := qovery.CLUSTERSTATEENUM_DEPLOYING
		MockDeployCluster(organization, cluster, &deployingState)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		err := service.DeployCluster("TestOrganization", "TestCluster", false)

		// then
		assert.Nil(t, err)
	})

	t.Run("Should deploy cluster with waiting for final status", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{*cluster})
		deployingState := qovery.CLUSTERSTATEENUM_DEPLOYING
		MockDeployCluster(organization, cluster, &deployingState)
		deployedState := qovery.CLUSTERSTATEENUM_DEPLOYED
		MockGetClusterStatus(organization, cluster, &deployedState)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		err := service.DeployCluster("TestOrganization", "TestCluster", true)

		// then
		assert.Nil(t, err)
	})
}

func TestStopManagedCluster(t *testing.T) {
	t.Run("Should stop cluster without waiting for final status", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{*cluster})
		deployingState := qovery.CLUSTERSTATEENUM_DEPLOYING
		MockStopCluster(organization, cluster, &deployingState)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		err := service.StopCluster("TestOrganization", "TestCluster", false)

		// then
		assert.Nil(t, err)
	})
	t.Run("Should stop cluster with waiting for final status", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		mockOrganization.MockListOrganizationsOk([]qovery.Organization{*organization})
		MockListClusters(organization, []qovery.Cluster{*cluster})
		deployingState := qovery.CLUSTERSTATEENUM_DEPLOYING
		MockStopCluster(organization, cluster, &deployingState)
		deployedState := qovery.CLUSTERSTATEENUM_DEPLOYED
		MockGetClusterStatus(organization, cluster, &deployedState)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		err := service.StopCluster("TestOrganization", "TestCluster", true)

		// then
		assert.Nil(t, err)
	})
}

func TestAskToEditStorageClass(t *testing.T) {
	t.Run("Should succeed to edit storage class", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		var storageClass = "current storage class"
		MockGetClusterAdvancedSettings(
			organization,
			cluster,
			&qovery.ClusterAdvancedSettings{
				StorageclassFastSsd: &storageClass,
			},
		)
		MockEditClusterAdvancedSettings(organization, cluster)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name": "new storage class",
				},
			),
		)

		// when
		err := service.AskToEditStorageClass(cluster)

		// then
		assert.Nil(t, err)
		var clusterAdvancedSettings = allAdvancedSettingsByClusterId[cluster.Id]
		assert.Equal(t, "new storage class", *clusterAdvancedSettings.StorageclassFastSsd)
	})
	t.Run("Should fail to edit storage class when storage class name prompt fails", func(t *testing.T) {
		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name": true,
				},
				map[string]string{},
			),
		)

		// when
		err := service.AskToEditStorageClass(cluster)

		// then
		assert.NotNil(t, err)
		assert.Equal(t, "error for prompt 'We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name'", err.Error())
	})
	t.Run("Should fail to edit storage class when storage class name is empty", func(t *testing.T) {
		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name": "",
				},
			),
		)

		// when
		err := service.AskToEditStorageClass(cluster)

		// then
		assert.NotNil(t, err)
		assert.Equal(t, "storage class name should be defined and cannot be empty", err.Error())
	})
}

func TestListClusterRegions(t *testing.T) {
	testCases := []struct {
		CloudProviderType qovery.CloudProviderEnum
		ExpectedRegions   []qovery.ClusterRegion
	}{
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_AWS, ExpectedRegions: []qovery.ClusterRegion{{Name: "eu-west-3", CountryCode: "FR", Country: "France", City: "Paris"}}},
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_SCW, ExpectedRegions: []qovery.ClusterRegion{{Name: "eu-west-3", CountryCode: "FR", Country: "France", City: "Paris"}}},
		{CloudProviderType: qovery.CLOUDPROVIDERENUM_GCP, ExpectedRegions: []qovery.ClusterRegion{{Name: "eu-west-3", CountryCode: "FR", Country: "France", City: "Paris"}}},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("Should succeed to list regions for cluster type %s", testCase.CloudProviderType), func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			// mocks
			MockListCloudProviderRegion(testCase.CloudProviderType, testCase.ExpectedRegions)

			// given
			service := NewClusterService(
				utils.GetQoveryClient("Fake token type", "Fake token"),
				promptuifactory.NewPromptUiFactoryMock(
					map[string]bool{},
					map[string]string{},
				),
			)

			// when
			regions, err := service.ListClusterRegions(testCase.CloudProviderType)

			// then
			assert.Nil(t, err)
			assert.Len(t, regions.Results, 1)
			var region = regions.Results[0]
			assert.Equal(t, testCase.ExpectedRegions[0].Name, region.Name)
			assert.Equal(t, testCase.ExpectedRegions[0].Country, region.Country)
			assert.Equal(t, testCase.ExpectedRegions[0].CountryCode, region.CountryCode)
			assert.Equal(t, testCase.ExpectedRegions[0].City, region.City)
		})
	}
	t.Run("Should trigger an error if cloud provider is On Premise as it is not handled", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		MockListCloudProviderRegion(qovery.CLOUDPROVIDERENUM_ON_PREMISE, nil)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{},
			),
		)

		// when
		regions, err := service.ListClusterRegions(qovery.CLOUDPROVIDERENUM_ON_PREMISE)

		// then
		assert.Nil(t, regions)
		assert.NotNil(t, err)
		assert.Equal(t, "cannot list regions for 'ON_PREMISE' cloud provider", err.Error())
	})
}

func TestGetHelmValues(t *testing.T) {
	t.Run("Should succeed to edit storage class", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = CreateTestCluster(organization)
		var storageClass = "current storage class"
		MockGetClusterAdvancedSettings(
			organization,
			cluster,
			&qovery.ClusterAdvancedSettings{
				StorageclassFastSsd: &storageClass,
			},
		)
		MockEditClusterAdvancedSettings(organization, cluster)

		// given
		service := NewClusterService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name": "new storage class",
				},
			),
		)

		// when
		err := service.AskToEditStorageClass(cluster)

		// then
		assert.Nil(t, err)
		var clusterAdvancedSettings = allAdvancedSettingsByClusterId[cluster.Id]
		assert.Equal(t, "new storage class", *clusterAdvancedSettings.StorageclassFastSsd)
	})
}