package usercontext

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/qovery/qovery-client-go"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

func GetOrganizationContextResourceId(qoveryAPIClient *qovery.APIClient, organizationName string) (string, error) {
	if strings.TrimSpace(organizationName) == "" {
		id, _, err := utils.CurrentOrganization(true)
		if err != nil {
			return "", err
		}

		return string(id), nil
	}

	// find organization by name
	organizations, _, err := qoveryAPIClient.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute()

	if err != nil {
		return "", err
	}

	organization := utils.FindByOrganizationName(organizations.GetResults(), organizationName)
	if organization == nil {
		return "", errors.Errorf("organization %s not found", organizationName)
	}

	return organization.Id, nil
}
