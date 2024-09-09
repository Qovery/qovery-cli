package containerregistry

import (
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	mockCluster "github.com/qovery/qovery-cli/pkg/cluster"
	mockOrganization "github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

func TestAskToEditGenericClusterContainerRegistry(t *testing.T) {
	t.Run("Should edit successfully cluster container registry", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)
		MockEditClusterContainerRegistry(organization, "id-container-registry-to-edit", false)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
				"Url of your registry":                      "https://my-registry.com",
				"Username to use to login to your registry": "foo",
				"Password to use to login to your registry": "bar",
			}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.Nil(t, err)
	})
	t.Run("Should fail if configure prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": true,
				},
				map[string]string{},
			),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if list container registries call fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		MockListClusterContainerRegistries(organization, []qovery.ContainerRegistryResponse{}, true)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
				},
			),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if configure prompt results in unhandled container registry type", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)
		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "Unhandled",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if url prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)
		// given

		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Url of your registry": true,
				},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if username prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Username to use to login to your registry": true,
				}, map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
					"Url of your registry": "https://my-registry.com",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if password prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Password to use to login to your registry": true,
				}, map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
					"Url of your registry":                      "https://my-registry.com",
					"Username to use to login to your registry": "foo",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
	t.Run("Should fail if edit container registry call fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)
		MockEditClusterContainerRegistry(organization, "id-container-registry-to-edit", true)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "A Generic One",
					"Url of your registry":                      "https://my-registry.com",
					"Username to use to login to your registry": "foo",
					"Password to use to login to your registry": "bar",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
	})
}

func TestAskToEditClusterGithubContainerRegistry(t *testing.T) {
	t.Run("Should edit successfully github cluster container registry", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)
		MockEditClusterContainerRegistry(organization, "id-container-registry-to-edit", false)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "Github",
				"Enter your Github username to login to the registry. It should be your Github username or Organisation name":                     "login_github",
				"Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions":  "token_github",
			}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.Nil(t, err)
	})
	t.Run("Should fail if login prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Enter your Github username to login to the registry. It should be your Github username or Organisation name": true,
				},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "Github",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
		assert.Equal(t, "error for prompt 'Enter your Github username to login to the registry. It should be your Github username or Organisation name'", err.Error())
	})
	t.Run("Should fail if password prompt fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks
		var organization = mockOrganization.CreateTestOrganization()
		var cluster = mockCluster.CreateTestCluster(organization)
		var existingContainerRegistry = qovery.NewContainerRegistryResponse("id-container-registry-to-edit", time.Now())
		existingContainerRegistry.SetName("container registry to edit")
		existingContainerRegistry.SetUrl("https://ecr-url.com")
		existingContainerRegistry.SetDescription("")
		existingContainerRegistry.SetUpdatedAt(time.Now())
		existingContainerRegistry.SetCluster(qovery.ContainerRegistryResponseAllOfCluster{Id: cluster.Id, Name: cluster.Name})
		existingContainerRegistry.SetKind(qovery.CONTAINERREGISTRYKINDENUM_ECR)

		MockListClusterContainerRegistries(
			organization,
			[]qovery.ContainerRegistryResponse{*existingContainerRegistry},
			false,
		)

		// given
		var service = NewClusterContainerRegistryService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions": true,
				},
				map[string]string{
					"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry": "Github",
					"Enter your Github username to login to the registry. It should be your Github username or Organisation name":                     "login_github",
				}),
		)

		// when
		var err = service.AskToEditClusterContainerRegistry(organization.Id, cluster.Id)

		// then
		assert.NotNil(t, err)
		assert.Equal(t, "error for prompt 'Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions'", err.Error())
	})
}