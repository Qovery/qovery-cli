package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Services struct {
	Results []Service `json:"results"`
}

type Service struct {
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Version     string       `json:"version"`
	Status      string       `json:"status"`
	Application *Application `json:"application"`
}

func ListServices(projectId string, branchId string, resourcePath string) Services {
	req, _ := http.NewRequest("GET", RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/branch/"+branchId+"/"+resourcePath, nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	services := Services{}

	if err != nil {
		return services
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &services)

	return services
}

func ListDatabases(projectId string, branchId string) Services {
	return ListServices(projectId, branchId, "database")
}

func ListBrokers(projectId string, branchId string) Services {
	return ListServices(projectId, branchId, "broker")
}

func ListStorage(projectId string, branchId string) Services {
	return ListServices(projectId, branchId, "storage")
}
