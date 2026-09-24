//go:build testing

package containerregistry

import (
	"encoding/json"
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/qovery/qovery-client-go"
	"net/http"
)

var allClusterContainerRegistryRequestsById = make(map[string]qovery.ContainerRegistryRequest)

func MockListClusterContainerRegistries(organization *qovery.Organization, containerRegistries []qovery.ContainerRegistryResponse, forceFail bool) {
	var response = qovery.ContainerRegistryResponseList{Results: containerRegistries}
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/containerRegistry")
	httpmock.RegisterResponder("GET", url,
		func(req *http.Request) (*http.Response, error) {
			if forceFail {
				return httpmock.NewStringResponse(500, "Force failed enabled"), nil
			}
			resp, err := httpmock.NewJsonResponse(200, response)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

func MockEditClusterContainerRegistry(organization *qovery.Organization, containerRegistryId string, forceFail bool) {
	var url = fmt.Sprint("https://api.qovery.com/organization/", organization.Id, "/containerRegistry/", containerRegistryId)
	httpmock.RegisterResponder("PUT", url,
		func(req *http.Request) (*http.Response, error) {
			if forceFail {
				return httpmock.NewStringResponse(500, "Force failed enabled"), nil
			}
			var containerRegistryRequest qovery.ContainerRegistryRequest
			if err := json.NewDecoder(req.Body).Decode(&containerRegistryRequest); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			allClusterContainerRegistryRequestsById[containerRegistryId] = containerRegistryRequest
			resp, err := httpmock.NewJsonResponse(200, qovery.ContainerRegistryResponse{
				Id:          containerRegistryId,
				Name:        &containerRegistryRequest.Name,
				Kind:        &containerRegistryRequest.Kind,
				Description: containerRegistryRequest.Description,
				Url:         containerRegistryRequest.Url,
			})
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		})
}

type ContainerRegistryServiceMock struct {
	ResultAskToEditClusterContainerRegistry error
}

func (mock *ContainerRegistryServiceMock) AskToEditClusterContainerRegistry(organizationId string, clusterId string) error {
	return mock.ResultAskToEditClusterContainerRegistry
}
