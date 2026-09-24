//go:build testing

package credentials

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
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
	url := fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/", cloudProviderTypeUrl, "/credentials")
	httpmock.RegisterResponder("POST", url,
		func(req *http.Request) (*http.Response, error) {
			// Read body bytes so we can decode twice (once into T, once into a map for name extraction)
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			// Decode & store the credentials request
			var credentials T
			if err := json.Unmarshal(bodyBytes, &credentials); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			generatedUuid := uuid.NewString()
			allCredentialsById[generatedUuid] = credentials

			// Extract name from the raw JSON (works for both flat and oneOf wrapper types)
			var rawMap map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			var credentialsName string
			if name, ok := rawMap["name"].(string); ok {
				credentialsName = name
			}
			var response qovery.ClusterCredentials
			switch cloudProviderTypeUrl {
			case "aws":
				response = qovery.ClusterCredentials{AwsStaticClusterCredentials: &qovery.AwsStaticClusterCredentials{
					Id:          generatedUuid,
					Name:        credentialsName,
					AccessKeyId: "",
					ObjectType:  "AWS",
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
	url := fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/", cloudProviderTypeUrl, "/credentials")
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
