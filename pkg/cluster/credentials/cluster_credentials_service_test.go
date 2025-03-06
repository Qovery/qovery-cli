package credentials

import (
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

func TestCredentialsNameOnCreateCredentials(t *testing.T) {
	t.Run("Should fail if issue happens when entering credentials name", func(t *testing.T) {
		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Give a name to your credentials": true,
				},
				map[string]string{}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(uuid.NewString(), qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "error for prompt 'Give a name to your credentials'", err.Error())
	})
	t.Run("Should fail if credentials name entered is empty", func(t *testing.T) {
		// given
		var emptyName = ""
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": emptyName,
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(uuid.NewString(), qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty name for your credentials", err.Error())
	})
	t.Run("Should fail if credentials name entered is empty on trim", func(t *testing.T) {
		// given
		var emptyOnTrimName = "  "
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": emptyOnTrimName,
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(uuid.NewString(), qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty name for your credentials", err.Error())
	})
}

func TestAwsCredentials(t *testing.T) {
	t.Run("Should succeed to create AWS credentials according to prompt user inputs", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateAwsCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "aws-credentials",
				"Enter your AWS access key":       "aws-access-key",
				"Enter your AWS secret key":       "aws-secret-key",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
		var createdCredentials = allCredentialsById[credentials.AwsStaticClusterCredentials.Id].(qovery.AwsCredentialsRequest).AwsStaticCredentialsRequest
		assert.Equal(t, "aws-credentials", createdCredentials.Name)
		assert.Equal(t, "aws-access-key", createdCredentials.AccessKeyId)
		assert.Equal(t, "aws-secret-key", createdCredentials.SecretAccessKey)
	})
	t.Run("Should fail to create AWS credentials if access key is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateAwsCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "aws-credentials",
				"Enter your AWS access key":       "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty access key", err.Error())
	})
	t.Run("Should fail to create AWS credentials if secret key is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateAwsCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "aws-credentials",
				"Enter your AWS access key":       "aws-access-key",
				"Enter your AWS secret key":       "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty secret key", err.Error())
	})
	t.Run("Should list AWS credentials", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockListCloudProviderCredentials(
			organization,
			&qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
				{AwsStaticClusterCredentials: &qovery.AwsStaticClusterCredentials{Id: "id", Name: "AWS Credentials"}},
			}},
			"aws",
		)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var credentials, err = service.ListClusterCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_AWS)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
	})
}

func TestScalewayCredentials(t *testing.T) {
	t.Run("Should succeed to create SCW credentials according to prompt user inputs", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateScalewayCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "scaleway-credentials",
				"Enter your SCW access key":       "scw-access-key",
				"Enter your SCW secret key":       "scw-secret-key",
				"Enter your SCW organization ID":  "scw-organization-id",
				"Enter your SCW project ID":       "scw-project-id",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
		var createdCredentials = allCredentialsById[credentials.ScalewayClusterCredentials.Id].(qovery.ScalewayCredentialsRequest)
		assert.Equal(t, "scaleway-credentials", createdCredentials.Name)
		assert.Equal(t, "scw-access-key", createdCredentials.ScalewayAccessKey)
		assert.Equal(t, "scw-secret-key", createdCredentials.ScalewaySecretKey)
		assert.Equal(t, "scw-organization-id", createdCredentials.ScalewayOrganizationId)
		assert.Equal(t, "scw-project-id", createdCredentials.ScalewayProjectId)
	})
	t.Run("Should fail to create SCW credentials if access key is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateScalewayCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "scaleway-credentials",
				"Enter your SCW access key":       "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty access key", err.Error())
	})
	t.Run("Should fail to create SCW credentials if secret key is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateScalewayCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "scaleway-credentials",
				"Enter your SCW access key":       "scw-access-key",
				"Enter your SCW secret key":       "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty secret key", err.Error())
	})
	t.Run("Should fail to create SCW credentials if organization id is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateScalewayCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "scaleway-credentials",
				"Enter your SCW access key":       "scw-access-key",
				"Enter your SCW secret key":       "scw-secret-key",
				"Enter your SCW organization ID":  "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty organization id", err.Error())
	})
	t.Run("Should fail to create SCW credentials if project id is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateScalewayCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials": "scaleway-credentials",
				"Enter your SCW access key":       "scw-access-key",
				"Enter your SCW secret key":       "scw-secret-key",
				"Enter your SCW organization ID":  "scw-organization-id",
				"Enter your SCW project ID":       "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty project id", err.Error())
	})
	t.Run("Should list SCW credentials", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockListCloudProviderCredentials(
			organization,
			&qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
				{ScalewayClusterCredentials: &qovery.ScalewayClusterCredentials{Id: "id", Name: "AWS Credentials"}},
			}},
			"scaleway",
		)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var credentials, err = service.ListClusterCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_SCW)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
	})
}

func TestGcpCredentials(t *testing.T) {
	t.Run("Should succeed to create GCP credentials according to prompt user inputs", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateGcpCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials":                    "gcp-credentials",
				"Enter your GCP JSON credentials (*base64* encoded)": "gcp-creds-json",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_GCP)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
		var createdCredentials = allCredentialsById[credentials.GenericClusterCredentials.Id].(qovery.GcpCredentialsRequest)
		assert.Equal(t, "gcp-credentials", createdCredentials.Name)
		assert.Equal(t, "gcp-creds-json", createdCredentials.GcpCredentials)
	})
	t.Run("Should fail to create GCP credentials if json is empty", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockCreateGcpCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{
				"Give a name to your credentials":                    "gcp-credentials",
				"Enter your GCP JSON credentials (*base64* encoded)": "",
			}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_GCP)

		// then
		assert.Nil(t, credentials)
		assert.NotNil(t, err)
		assert.Equal(t, "please enter a non-empty gcp json credentials", err.Error())
	})
	t.Run("Should list GCP credentials", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockListCloudProviderCredentials(
			organization,
			&qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
				{GenericClusterCredentials: &qovery.GenericClusterCredentials{Id: "id", Name: "AWS Credentials"}},
			}},
			"gcp",
		)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var credentials, err = service.ListClusterCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_GCP)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
	})
}

func TestOnPremiseOnCreateCredentials(t *testing.T) {
	t.Run("Should create automatically credentials with name 'on-premise' when creating on premise cluster credentials", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockOnPremiseCreateCredentials(organization)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var credentials, err = service.AskToCreateCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_ON_PREMISE)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
		var createdCredentials = allCredentialsById[credentials.GenericClusterCredentials.Id].(qovery.OnPremiseCredentialsRequest)
		assert.Equal(t, "on-premise", createdCredentials.Name)
	})
	t.Run("Should list On Premise credentials", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mock
		var organization = organization.CreateTestOrganization()
		MockListCloudProviderCredentials(
			organization,
			&qovery.ClusterCredentialsResponseList{Results: []qovery.ClusterCredentials{
				{GenericClusterCredentials: &qovery.GenericClusterCredentials{Id: "id", Name: "On Premise Credentials"}},
			}},
			"onPremise",
		)

		// given
		var service = NewClusterCredentialsService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		var credentials, err = service.ListClusterCredentials(organization.Id, qovery.CLOUDPROVIDERENUM_ON_PREMISE)

		// then
		assert.Nil(t, err)
		assert.NotNil(t, credentials)
	})
}
