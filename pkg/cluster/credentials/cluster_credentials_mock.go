//go:build testing

package credentials

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"net/http"
	"reflect"
)

// Stores credentials on POST for assert test purposes
var allCredentialsById = make(map[string]interface{})

// MockCreateAwsCredentials
// Stores the request into a hashmap to keep track, and return the response with the generated uuid
func MockCreateAwsCredentials(organization *qovery.Organization) {
	mockCreateCloudProviderCredentials[qovery.AwsCredentialsRequest](organization, "aws")
}

// MockCreateScalewayCredentials
// Stores the request into a hashmap to keep track, and return the response with the generated uuid
func MockCreateScalewayCredentials(organization *qovery.Organization) {
	mockCreateCloudProviderCredentials[qovery.ScalewayCredentialsRequest](organization, "scaleway")
}

// MockCreateGcpCredentials
// Stores the request into a hashmap to keep track, and return the response with the generated uuid
func MockCreateGcpCredentials(organization *qovery.Organization) {
	mockCreateCloudProviderCredentials[qovery.GcpCredentialsRequest](organization, "gcp")
}

// MockOnPremiseCreateCredentials
// Stores the request into a hashmap to keep track, and return the response with the generated uuid
func MockOnPremiseCreateCredentials(organization *qovery.Organization) {
	mockCreateCloudProviderCredentials[qovery.OnPremiseCredentialsRequest](organization, "onPremise")
}

func mockCreateCloudProviderCredentials[T any](organization *qovery.Organization, cloudProviderTypeUrl string) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/", cloudProviderTypeUrl, "/credentials")
	httpmock.RegisterResponder("POST", url,
		func(req *http.Request) (*http.Response, error) {
			// Decode & store the credentials request
			var credentials T
			if err := json.NewDecoder(req.Body).Decode(&credentials); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			generatedUuid := uuid.NewString()
			allCredentialsById[generatedUuid] = credentials

			var credentialsName = reflect.ValueOf(credentials).FieldByName("Name").String()
			var response qovery.ClusterCredentials
			switch cloudProviderTypeUrl {
			case "aws":
				response = qovery.ClusterCredentials{AwsStaticClusterCredentials: &qovery.AwsStaticClusterCredentials{
					Id:         generatedUuid,
					Name:       credentialsName,
					ObjectType: "AWS",
				}}
			case "scaleway":
				response = qovery.ClusterCredentials{ScalewayClusterCredentials: &qovery.ScalewayClusterCredentials{
					Id:         generatedUuid,
					Name:       credentialsName,
					ObjectType: "SCW",
				}}
			default:
				response = qovery.ClusterCredentials{GenericClusterCredentials: &qovery.GenericClusterCredentials{
					Id:         generatedUuid,
					Name:       credentialsName,
					ObjectType: "OTHER",
				}}
			}
			resp, err := httpmock.NewJsonResponse(200, response)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockListCloudProviderCredentials(organization *qovery.Organization, results *qovery.ClusterCredentialsResponseList, cloudProviderTypeUrl string) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/", cloudProviderTypeUrl, "/credentials")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, results)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

type ClusterCredentialsServiceMock struct {
	ResultListClusterCredentials func() (*qovery.ClusterCredentialsResponseList, error)
	ResultAskToCreateCredentials func() (*qovery.ClusterCredentials, error)
}

func (mock *ClusterCredentialsServiceMock) ListClusterCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentialsResponseList, error) {
	return mock.ResultListClusterCredentials()
}
func (mock *ClusterCredentialsServiceMock) AskToCreateCredentials(organizationID string, cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterCredentials, error) {
	return mock.ResultAskToCreateCredentials()
}
