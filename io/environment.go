package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Environments struct {
	Results []Environment `json:"results"`
}

type Environment struct {
	Id                  string              `json:"id"`
	Name                string              `json:"name"`
	Status              Status              `json:"status"`
	TotalApplications   *int                `json:"total_applications"`
	TotalServices       *int                `json:"total_services"`
	TotalDatabases      *int                `json:"total_databases"`
	TotalBrokers        *int                `json:"total_brokers"`
	CloudProviderRegion CloudProviderRegion `json:"cloud_provider_region"`
	Applications        []Application       `json:"applications"`
	Databases           []Service           `json:"databases"`
	Routers             []Router            `json:"routers"`
}

func (e *Environment) GetApplicationNames() []string {
	var names []string

	for _, x := range e.Applications {
		names = append(names, x.Name)
	}

	return names
}

func (e *Environment) GetDatabaseNames() []string {
	var names []string

	for _, x := range e.Databases {
		names = append(names, x.Name)
	}

	return names
}

func (e *Environment) GetApplication(name string) Application {
	for _, a := range e.Applications {
		if a.Name == name {
			return a
		}
	}

	return Application{}
}

func (e *Environment) GetConnectionURIs() []string {
	var uris []string
	for _, r := range e.Routers {
		for _, cd := range r.CustomDomains {
			if cd.Status.State == "LIVE" {
				uris = append(uris, cd.Domain)
			}
		}

		uris = append(uris, r.ConnectionURI)
	}

	return uris
}

func GetEnvironmentByName(projectId string, name string) Environment {
	for _, v := range ListEnvironments(projectId).Results {
		if v.Name == name {
			return v
		}
	}

	return Environment{}
}

func ListEnvironments(projectId string) Environments {
	r := Environments{}

	if projectId == "" {
		return r
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return r
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}

func DeleteEnvironment(projectId string, environmentId string) {
	if projectId == "" || environmentId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/deploy", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
