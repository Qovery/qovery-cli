package api

import (
	"log"
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
	FQDN        string       `json:"fqdn"`
	Port        *int         `json:"port"`
	Username    string       `json:"username"`
	Password    string       `json:"password"`
	Application *Application `json:"application"`
}

func ListServices(projectId string, branchName string, resourcePath string) Services {
	CheckAuthenticationOrQuitWithMessage()
	var services Services
	if err := NewRequest(http.MethodGet, "/project/%s/branch/%s/%s", projectId, branchName, resourcePath).Do(&services); err != nil {
		log.Fatal(errorUnknownError)
	}
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
	var itf map[string]interface{}
	if err := NewRequest(http.MethodGet, "/project/%s/branch/%s/%s", projectId, branchName, resourcePath).Do(&itf); err != nil {
		log.Fatal(errorUnknownError)
	}
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
