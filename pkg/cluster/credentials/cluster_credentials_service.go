package credentials

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/qovery/qovery-client-go"
	"io"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

type ClusterCredentialsService interface {
	ListClusterCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentialsResponseList, error)
	AskToCreateCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum, ) (*qovery.ClusterCredentials, error)
}

type ClusterCredentialsServiceImpl struct {
	client          *qovery.APIClient
	promptUiFactory promptuifactory.PromptUiFactory
}

func NewClusterCredentialsService(
	client *qovery.APIClient,
	promptUiFactory promptuifactory.PromptUiFactory,
) *ClusterCredentialsServiceImpl {
	return &ClusterCredentialsServiceImpl{
		client,
		promptUiFactory,
	}
}

func (service *ClusterCredentialsServiceImpl) ListClusterCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentialsResponseList, error) {
	switch cloudProviderType {
	case qovery.CLOUDPROVIDERENUM_GCP:
		req := service.client.CloudProviderCredentialsAPI.ListGcpCredentials(context.Background(), organizationID)
		creds, _, err := service.client.CloudProviderCredentialsAPI.ListGcpCredentialsExecute(req)
		if err != nil {
			return nil, err
		}
		return creds, nil
	case qovery.CLOUDPROVIDERENUM_AWS:
		req := service.client.CloudProviderCredentialsAPI.ListAWSCredentials(context.Background(), organizationID)
		creds, _, err := service.client.CloudProviderCredentialsAPI.ListAWSCredentialsExecute(req)
		if err != nil {
			return nil, err
		}
		return creds, nil
	case qovery.CLOUDPROVIDERENUM_SCW:
		req := service.client.CloudProviderCredentialsAPI.ListScalewayCredentials(context.Background(), organizationID)
		creds, _, err := service.client.CloudProviderCredentialsAPI.ListScalewayCredentialsExecute(req)
		if err != nil {
			return nil, err
		}
		return creds, nil
	case qovery.CLOUDPROVIDERENUM_ON_PREMISE:
		req := service.client.CloudProviderCredentialsAPI.ListOnPremiseCredentials(context.Background(), organizationID)
		creds, _, err := service.client.CloudProviderCredentialsAPI.ListOnPremiseCredentialsExecute(req)
		if err != nil {
			return nil, err
		}
		return creds, nil
	default:
		return nil, fmt.Errorf("cannot list credentias for '%s' cloud provider type", cloudProviderType)
	}
}

func (service *ClusterCredentialsServiceImpl) AskToCreateCredentials(
	organizationID string,
	cloudProviderType qovery.CloudProviderEnum,
) (*qovery.ClusterCredentials, error) {
	// Early return for ON_PREMISE cloud provider
	// As the name of the credentials is forced to the value "on-premise", no need to require user to enter some credentials name
	if cloudProviderType == qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		creds, resp, err := service.client.CloudProviderCredentialsAPI.CreateOnPremiseCredentials(context.Background(), organizationID).OnPremiseCredentialsRequest(qovery.OnPremiseCredentialsRequest{
			Name: "on-premise",
		}).Execute()
		if err != nil || resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("%s: %v\n%s\n", color.RedString("Error"), string(body), err)
		}
		return creds, nil
	}

	// Normal path
	credentialsName, err := service.promptUiFactory.RunPrompt("Give a name to your credentials", "")
	if err != nil {
		return nil, err
	}

	// Check if credentials name is not empty or blank
	if utils.IsEmptyOrBlank(credentialsName) {
		return nil, fmt.Errorf("please enter a non-empty name for your credentials")
	}

	switch cloudProviderType {
	case qovery.CLOUDPROVIDERENUM_AWS:
		accessKey, err := service.promptUiFactory.RunPrompt("Enter your AWS access key", "")
		if err != nil {
			return nil, err
		}
		secretKey, err := service.promptUiFactory.RunPrompt("Enter your AWS secret key", "")
		if err != nil {
			return nil, err
		}

		if utils.IsEmptyOrBlank(accessKey) {
			return nil, fmt.Errorf("please enter a non-empty access key")
		}

		if utils.IsEmptyOrBlank(secretKey) {
			return nil, fmt.Errorf("please enter a non-empty secret key")
		}

		creds, resp, err := service.client.CloudProviderCredentialsAPI.CreateAWSCredentials(context.Background(), organizationID).AwsCredentialsRequest(qovery.AwsCredentialsRequest{
			Name:            credentialsName,
			AccessKeyId:     accessKey,
			SecretAccessKey: secretKey,
		}).Execute()
		if err != nil || resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("%s: %v\n%s\n", color.RedString("Error"), string(body), err)
		}
		return creds, nil

	case qovery.CLOUDPROVIDERENUM_SCW:
		accessKey, err := service.promptUiFactory.RunPrompt("Enter your SCW access key", "")
		if err != nil {
			return nil, err
		}
		secretKey, err := service.promptUiFactory.RunPrompt("Enter your SCW secret key", "")
		if err != nil {
			return nil, err
		}
		organizationId, err := service.promptUiFactory.RunPrompt("Enter your SCW organization ID", "")
		if err != nil {
			return nil, err
		}
		projectId, err := service.promptUiFactory.RunPrompt("Enter your SCW project ID", "")
		if err != nil {
			return nil, err
		}

		if utils.IsEmptyOrBlank(accessKey) {
			return nil, fmt.Errorf("please enter a non-empty access key")
		}

		if utils.IsEmptyOrBlank(secretKey) {
			return nil, fmt.Errorf("please enter a non-empty secret key")
		}

		if utils.IsEmptyOrBlank(organizationId) {
			return nil, fmt.Errorf("please enter a non-empty organization id")
		}

		if utils.IsEmptyOrBlank(projectId) {
			return nil, fmt.Errorf("please enter a non-empty project id")
		}

		creds, resp, err := service.client.CloudProviderCredentialsAPI.CreateScalewayCredentials(context.Background(), organizationID).ScalewayCredentialsRequest(qovery.ScalewayCredentialsRequest{
			Name:                   credentialsName,
			ScalewayAccessKey:      accessKey,
			ScalewaySecretKey:      secretKey,
			ScalewayProjectId:      projectId,
			ScalewayOrganizationId: organizationId,
		}).Execute()
		if err != nil || resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("%s: %v\n%s\n", color.RedString("Error"), string(body), err)
		}
		return creds, nil

	case qovery.CLOUDPROVIDERENUM_GCP:
		gcpJsonCredentials, err := service.promptUiFactory.RunPrompt("Enter your GCP JSON credentials (*base64* encoded)", "")
		if err != nil {
			return nil, err
		}
		if utils.IsEmptyOrBlank(gcpJsonCredentials) {
			return nil, fmt.Errorf("please enter a non-empty gcp json credentials")
		}
		creds, resp, err := service.client.CloudProviderCredentialsAPI.CreateGcpCredentials(context.Background(), organizationID).GcpCredentialsRequest(qovery.GcpCredentialsRequest{
			Name:           credentialsName,
			GcpCredentials: gcpJsonCredentials,
		}).Execute()
		if err != nil || resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("%s: %v\n%s\n", color.RedString("Error"), string(body), err)
		}
		return creds, nil
	}

	return nil, fmt.Errorf("unhandled cloud provider type during credentials creation: %s", cloudProviderType)
}

