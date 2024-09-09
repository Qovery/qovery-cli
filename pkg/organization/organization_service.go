package organization

import (
	"context"
	"errors"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"strings"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
)

type OrganizationDto struct {
	ID   string
	Name string
}

type OrganizationService interface {
	AskUserToSelectOrganization() (*OrganizationDto, error)
}

type OrganizationServiceImpl struct {
	client          *qovery.APIClient
	promptUiFactory promptuifactory.PromptUiFactory
}

func NewOrganizationService(client *qovery.APIClient, promptUiFactory promptuifactory.PromptUiFactory) *OrganizationServiceImpl {
	return &OrganizationServiceImpl{
		client,
		promptUiFactory,
	}
}

func (service *OrganizationServiceImpl) AskUserToSelectOrganization() (*OrganizationDto, error) {
	organizations, res, err := service.client.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute()
	if err != nil || res.StatusCode >= 400 {
		return nil, fmt.Errorf("Error when listing organizations: %s (response status = %s)", err, res.Status)
	}

	var organizationNames []string
	var orgs = make(map[string]string)

	for _, org := range organizations.GetResults() {
		organizationNames = append(organizationNames, org.Name)
		orgs[org.Name] = org.Id
	}

	if len(organizationNames) < 1 {
		return nil, errors.New("No organization found.")
	}

	if len(organizationNames) == 1 {
		return &OrganizationDto{
			ID:   orgs[organizationNames[0]],
			Name: organizationNames[0],
		}, nil
	}

	fmt.Println("Organization:")
	_, selectedOrganization, err := service.promptUiFactory.RunSelectWithSizeAndSearcher(
		"Organization",
		organizationNames,
		30,
		func(input string, index int) bool {
			return strings.Contains(strings.ToLower(organizationNames[index]), strings.ToLower(input))
		},
	)
	if err != nil {
		return nil, err
	}

	return &OrganizationDto{
		ID:   orgs[selectedOrganization],
		Name: selectedOrganization,
	}, nil
}
