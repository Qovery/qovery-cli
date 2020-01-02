package api

import (
	"log"
	"net/http"
)

type Applications struct {
	Results []Application `json:"results"`
}

type Application struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	TotalDatabases *int   `json:"total_databases"`
	TotalBrokers   *int   `json:"total_brokers"`
	TotalStorage   *int   `json:"total_storage"`
}

func ListApplications(projectId string, branchName string) Applications {
	CheckAuthenticationOrQuitWithMessage()
	var apps Applications
	if err := NewRequest(http.MethodGet, "/user/%s/project/%s/branch/%s/application",
		GetAccountId(), projectId, branchName).Do(&apps); err != nil {
		log.Fatal(errorUnknownError)
	}
	return apps
}
