package containerregistry

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/fatih/color"
	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
)

type ClusterContainerRegistryService interface {
	AskToEditClusterContainerRegistry(organizationId string, clusterId string) error
}

type ClusterContainerRegistryServiceImpl struct {
	client          *qovery.APIClient
	promptUiFactory promptuifactory.PromptUiFactory
}

func NewClusterContainerRegistryService(
	client *qovery.APIClient,
	promptUiFactory promptuifactory.PromptUiFactory,
) *ClusterContainerRegistryServiceImpl {
	return &ClusterContainerRegistryServiceImpl{
		client,
		promptUiFactory,
	}
}

func (service *ClusterContainerRegistryServiceImpl) AskToEditClusterContainerRegistry(organizationId string, clusterId string) error {
	_, configureContainerRegistry, err := service.promptUiFactory.RunSelect(
		"You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry",
		[]string{"Github", "A Generic One"},
	)

	if err != nil {
		return err
	}

	resp, _, err := service.client.ContainerRegistriesAPI.ListContainerRegistry(context.Background(), organizationId).Execute()
	if err != nil {
		return err
	}

	// Only 1 container registry exists for a Self Managed cluster, so select it according to the Cluster Id
	indexSelfManagedClusterRegistry := slices.IndexFunc(resp.GetResults(), func(c qovery.ContainerRegistryResponse) bool { return c.Cluster != nil && c.Cluster.Id == clusterId })
	selfManagedClusterRegistry := resp.Results[indexSelfManagedClusterRegistry]

	var registryInfo *AskRegistryInfo
	switch configureContainerRegistry {
	case "Github":
		registryInfo, err = service.askGithubRegistryInfo()
		if err != nil {
			return err
		}
	case "A Generic One":
		registryInfo, err = service.askGenericRegistryInfo(selfManagedClusterRegistry.Kind)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot configure container registry: %s", configureContainerRegistry)
	}

	_, res, err := service.client.ContainerRegistriesAPI.EditContainerRegistry(context.Background(), organizationId, selfManagedClusterRegistry.Id).ContainerRegistryRequest(qovery.ContainerRegistryRequest{
		Name:        *selfManagedClusterRegistry.Name,
		Kind:        registryInfo.Kind,
		Description: selfManagedClusterRegistry.Description,
		Url:         &registryInfo.Url,
		Config: qovery.ContainerRegistryRequestConfig{
			Username: &registryInfo.Login,
			Password: &registryInfo.Password,
		},
	}).Execute()

	if err != nil {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s: %v", color.RedString("Error"), string(body))
	}

	return nil
}

type AskRegistryInfo struct {
	Url      string
	Login    string
	Password string
	Kind     qovery.ContainerRegistryKindEnum
}

func (service *ClusterContainerRegistryServiceImpl) askGithubRegistryInfo() (*AskRegistryInfo, error) {
	login, err := service.promptUiFactory.RunPrompt("Enter your Github username to login to the registry. It should be your Github username or Organisation name", "")
	if err != nil {
		return nil, err
	}

	password, err := service.promptUiFactory.RunPrompt("Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions", "")
	if err != nil {
		return nil, err
	}

	return &AskRegistryInfo{
		Url:      "https://ghcr.io",
		Login:    login,
		Password: password,
		Kind:     qovery.CONTAINERREGISTRYKINDENUM_GITHUB_CR,
	}, nil
}

func (service *ClusterContainerRegistryServiceImpl) askGenericRegistryInfo(clusterSelfManagedRegistryKind *qovery.ContainerRegistryKindEnum) (*AskRegistryInfo, error) {
	url, err := service.promptUiFactory.RunPrompt("Url of your registry", "https://")
	if err != nil {
		return nil, err
	}

	login, err := service.promptUiFactory.RunPrompt("Username to use to login to your registry", "")
	if err != nil {
		return nil, err
	}

	password, err := service.promptUiFactory.RunPrompt("Password to use to login to your registry", "")
	if err != nil {
		return nil, err
	}

	return &AskRegistryInfo{
		Url:      url,
		Login:    login,
		Password: password,
		Kind:     *clusterSelfManagedRegistryKind,
	}, nil
}
