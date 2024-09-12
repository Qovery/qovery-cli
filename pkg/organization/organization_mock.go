//go:build testing

package organization

import (
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"net/http"
	"time"
)

var testOrganizationId = "00000000-0000-0000-0000-000000000000"

// CreateTestOrganization Used to create one single organization with predefined values
func CreateTestOrganization() *qovery.Organization {
	return qovery.NewOrganization(testOrganizationId, time.Now(), "TestOrganization", qovery.PLANENUM_FREE)
}

// CreateRandomTestOrganization Used to create a few organizations with random values for ID and name
func CreateRandomTestOrganization() *qovery.Organization {
	return qovery.NewOrganization(uuid.NewString(), time.Now(), uuid.NewString(), qovery.PLANENUM_FREE)
}

func MockListOrganizationsOk(organizations []qovery.Organization) {
	var listOrganizationsResponse = qovery.OrganizationResponseList{Results: organizations}
	httpmock.RegisterResponder("GET", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, listOrganizationsResponse)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockListOrganizationsBadRequest() {
	httpmock.RegisterResponder("GET", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(400, "Bad Request"), nil
		})
}

type OrganizationServiceMock struct {
	ResultAskUserToSelectOrganization func() (*OrganizationDto, error)
}

func (mock *OrganizationServiceMock) AskUserToSelectOrganization() (*OrganizationDto, error) {
	return mock.ResultAskUserToSelectOrganization()
}

