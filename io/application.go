package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Applications struct {
	Results []Application `json:"results"`
}

type Application struct {
	Id             string     `json:"id"`
	Name           string     `json:"name"`
	Status         Status     `json:"status"`
	ConnectionURI  string     `json:"connection_uri"`
	TotalDatabases *int       `json:"total_databases"`
	TotalBrokers   *int       `json:"total_brokers"`
	Databases      []Service  `json:"databases"`
	Brokers        []Service  `json:"brokers"`
	Repository     Repository `json:"repository"`
}

func GetApplicationByName(projectId string, environmentId string, name string) Application {
	for _, a := range ListApplications(projectId, environmentId).Results {
		if a.Name == name {
			return a
		}
	}

	return Application{}
}

func (a *Application) GetDatabaseNames() []string {
	var names []string

	for _, x := range a.Databases {
		names = append(names, x.Name)
	}

	return names
}

func (a *Application) GetBrokerNames() []string {
	var names []string

	for _, x := range a.Brokers {
		names = append(names, x.Name)
	}

	return names
}

func ListApplications(projectId string, environmentId string) Applications {
	apps := Applications{}

	if projectId == "" || environmentId == "" {
		return apps
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/application", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return apps
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &apps)

	return apps
}
