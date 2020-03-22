package api

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
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Version     string       `json:"version"`
	Status      Status       `json:"status"`
	FQDN        string       `json:"fqdn"`
	Port        *int         `json:"port"`
	Username    string       `json:"username"`
	Password    string       `json:"password"`
	Application *Application `json:"application"`
}

func ListServices(projectId string, branchName string, resourcePath string) Services {
	services := Services{}

	if projectId == "" || branchName == "" || resourcePath == "" {
		return services
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/branch/"+branchName+"/"+resourcePath, nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err != nil {
		return services
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &services)

	return services
}

func ListDatabases(projectId string, branchName string) Services {
	return ListServices(projectId, branchName, "database")
}

func ListBrokers(projectId string, branchName string) Services {
	return ListServices(projectId, branchName, "broker")
}

func ListStorage(projectId string, branchName string) Services {
	return ListServices(projectId, branchName, "storage")
}

func ListServicesRaw(projectId string, branchName string, resourcePath string) map[string]interface{} {
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/branch/"+branchName+"/"+resourcePath, nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	itf := map[string]interface{}{}

	if err != nil {
		return itf
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &itf)

	return itf
}

func ListDatabasesRaw(projectId string, branchName string) map[string]interface{} {
	return ListServicesRaw(projectId, branchName, "database")
}

func ListBrokersRaw(projectId string, branchName string) map[string]interface{} {
	return ListServicesRaw(projectId, branchName, "broker")
}

func ListStorageRaw(projectId string, branchName string) map[string]interface{} {
	return ListServicesRaw(projectId, branchName, "storage")
}

func ListApplicationsRaw(projectId string, branchName string) map[string]interface{} {
	return ListServicesRaw(projectId, branchName, "application")
}
