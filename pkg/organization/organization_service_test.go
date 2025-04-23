package organization

import (
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

func TestAskUserToSelectOrganization(t *testing.T) {
	t.Run("Should list organizations and select the correct one", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		var organization1 = CreateRandomTestOrganization()
		var organization2 = CreateRandomTestOrganization()
		MockListOrganizationsOk([]qovery.Organization{*organization1, *organization2})

		// given
		// mock promptui organization to be the organization2 name
		var promptUiExpectedValueByLabel = map[string]string{
			"Organization": organization2.Name,
		}
		var organizationService = NewOrganizationService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, promptUiExpectedValueByLabel),
		)

		// when
		var selectedOrganization, err = organizationService.AskUserToSelectOrganization()

		// then
		assert.Nil(t, err)
		assert.Equal(t, selectedOrganization.ID, organization2.Id)
	})
	t.Run("Should select the only organization present when necessary", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		var organization = CreateTestOrganization()
		MockListOrganizationsOk([]qovery.Organization{*organization})

		// given
		var organizationService = NewOrganizationService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		selectedOrganization, err := organizationService.AskUserToSelectOrganization()

		// then
		assert.Nil(t, err)
		assert.Equal(t, selectedOrganization.ID, organization.Id)
	})
	t.Run("Should fail if response returns bad request", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		MockListOrganizationsBadRequest()

		// given
		var organizationService = NewOrganizationService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		_, err := organizationService.AskUserToSelectOrganization()

		// then
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "Error when listing organizations: 400 Bad Request (response status = 400 Bad Request)")
	})
	t.Run("Should fail if no organization found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		MockListOrganizationsOk([]qovery.Organization{})

		// given
		var organizationService = NewOrganizationService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(map[string]bool{}, map[string]string{}),
		)

		// when
		_, err := organizationService.AskUserToSelectOrganization()

		// then
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "No organization found.")
	})
	t.Run("Should fail if prompt to select organization fails", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// mocks part
		var organization1 = CreateRandomTestOrganization()
		var organization2 = CreateRandomTestOrganization()
		MockListOrganizationsOk([]qovery.Organization{*organization1, *organization2})

		// given
		var organizationService = NewOrganizationService(
			utils.GetQoveryClient("Fake token type", "Fake token"),
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Organization": true,
				},
				map[string]string{},
			),
		)

		// when
		_, err := organizationService.AskUserToSelectOrganization()

		// then
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "error for select 'Organization'")
	})
}
