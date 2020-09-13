package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Services struct {
	Results []Service `json:"results"`
}

type Service struct {
	Id           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Version      string           `json:"version"`
	Status       DeploymentStatus `json:"status"`
	FQDN         string           `json:"fqdn"`
	Port         *int             `json:"port"`
	Username     string           `json:"username"`
	Password     string           `json:"password"`
	Applications []Application    `json:"applications"`
}

func (s *Service) GetApplicationNames() []string {
	var names []string

	for _, a := range s.Applications {
		names = append(names, a.Name)
	}

	return names
}

func ListServices(projectId string, environmentId string, resourcePath string) Services {
	services := Services{}

	if projectId == "" || environmentId == "" || resourcePath == "" {
		return services
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/"+resourcePath, nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return services
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &services)

	return services
}

func ListDatabases(projectId string, environmentId string) Services {
	return ListServices(projectId, environmentId, "database")
}

func ListBrokers(projectId string, environmentId string) Services {
	return ListServices(projectId, environmentId, "broker")
}

func ListServicesRaw(projectId string, environmentId string, resourcePath string) map[string]interface{} {
	itf := map[string]interface{}{}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/"+resourcePath, nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return itf
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &itf)

	return itf
}

func ListDatabasesRaw(projectId string, environmentId string) map[string]interface{} {
	return ListServicesRaw(projectId, environmentId, "database")
}

func ListBrokersRaw(projectId string, environmentId string) map[string]interface{} {
	return ListServicesRaw(projectId, environmentId, "broker")
}

func ListApplicationsRaw(projectId string, environmentId string) map[string]interface{} {
	return ListServicesRaw(projectId, environmentId, "application")
}
