package credentials

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/fatih/color"
	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

const (
	gcpCredentialsTypeWif            = "Workload Identity Federation"
	gcpCredentialsTypeServiceAccount = "Service Account JSON Key"
)

type ClusterCredentialsService interface {
	ListClusterCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentialsResponseList, error)
	AskToCreateCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentials, error)
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
		if apiErr := formatCloudProviderCredentialsApiError(resp, err); apiErr != nil {
			return nil, apiErr
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
			AwsStaticCredentialsRequest: &qovery.AwsStaticCredentialsRequest{
				Type:            "AWS_STATIC",
				Name:            credentialsName,
				AccessKeyId:     accessKey,
				SecretAccessKey: secretKey,
			},
		}).Execute()
		if apiErr := formatCloudProviderCredentialsApiError(resp, err); apiErr != nil {
			return nil, apiErr
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
		if apiErr := formatCloudProviderCredentialsApiError(resp, err); apiErr != nil {
			return nil, apiErr
		}
		return creds, nil

	case qovery.CLOUDPROVIDERENUM_GCP:
		_, gcpCredentialsType, err := service.promptUiFactory.RunSelect("Which GCP credentials type do you want to use?", []string{
			gcpCredentialsTypeWif,
			gcpCredentialsTypeServiceAccount,
		})
		if err != nil {
			return nil, err
		}

		var gcpCredentialsRequest qovery.GcpCredentialsRequest
		switch gcpCredentialsType {
		case gcpCredentialsTypeWif:
			serviceAccountEmail, err := service.promptUiFactory.RunPrompt("Enter your GCP service account email", "")
			if err != nil {
				return nil, err
			}
			workloadIdentityProviderResource, err := service.promptUiFactory.RunPrompt("Enter your GCP Workload Identity provider resource", "")
			if err != nil {
				return nil, err
			}
			if utils.IsEmptyOrBlank(serviceAccountEmail) {
				return nil, fmt.Errorf("please enter a non-empty gcp service account email")
			}
			if utils.IsEmptyOrBlank(workloadIdentityProviderResource) {
				return nil, fmt.Errorf("please enter a non-empty gcp workload identity provider resource")
			}

			gcpCredentialsRequest = qovery.GcpWorkloadIdentityFederationCredentialsRequestAsGcpCredentialsRequest(
				qovery.NewGcpWorkloadIdentityFederationCredentialsRequest(credentialsName, serviceAccountEmail, workloadIdentityProviderResource),
			)

		case gcpCredentialsTypeServiceAccount:
			gcpJsonCredentials, err := service.promptUiFactory.RunPrompt("Enter your GCP JSON credentials (*base64* encoded)", "")
			if err != nil {
				return nil, err
			}
			if utils.IsEmptyOrBlank(gcpJsonCredentials) {
				return nil, fmt.Errorf("please enter a non-empty gcp json credentials")
			}

			gcpServiceAccountKeyRequest := qovery.NewGcpServiceAccountKeyCredentialsRequest(credentialsName, gcpJsonCredentials)
			gcpCredentialsRequest = qovery.GcpServiceAccountKeyCredentialsRequestAsGcpCredentialsRequest(gcpServiceAccountKeyRequest)

		default:
			return nil, fmt.Errorf("unhandled gcp credentials type during credentials creation: %s", gcpCredentialsType)
		}

		creds, resp, err := service.client.CloudProviderCredentialsAPI.CreateGcpCredentials(context.Background(), organizationID).GcpCredentialsRequest(gcpCredentialsRequest).Execute()
		if apiErr := formatCloudProviderCredentialsApiError(resp, err); apiErr != nil {
			return nil, apiErr
		}
		return creds, nil
	}

	return nil, fmt.Errorf("unhandled cloud provider type during credentials creation: %s", cloudProviderType)
}

func formatCloudProviderCredentialsApiError(resp *http.Response, err error) error {
	if err == nil && (resp == nil || resp.StatusCode < http.StatusBadRequest) {
		return nil
	}

	if resp != nil && resp.Body != nil {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return fmt.Errorf("%s: %v\n%s", color.RedString("Error"), string(body), err)
		}
	}

	return fmt.Errorf("%s: %v", color.RedString("Error"), err)
}
