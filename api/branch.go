package api

import (
	"log"
	"net/http"
)

type AggregatedEnvironments struct {
	Results []AggregatedEnvironment `json:"results"`
}

type AggregatedEnvironment struct {
	BranchId          string   `json:"branch_id"`
	Status            string   `json:"status"`
	ConnectionURIs    []string `json:"connection_uris"`
	TotalApplications *int     `json:"total_applications"`
	TotalDatabases    *int     `json:"total_databases"`
	TotalBrokers      *int     `json:"total_brokers"`
	TotalStorage      *int     `json:"total_storage"`
}

func GetBranchByName(projectId string, name string) *AggregatedEnvironment {
	for _, v := range ListBranches(projectId).Results {
		if v.BranchId == name {
			return &v
		}
	}

	return nil
}

func ListBranches(projectId string) AggregatedEnvironments {
	CheckAuthenticationOrQuitWithMessage()
	var envs AggregatedEnvironments
	if err := NewRequest(http.MethodGet, "/project/%s/branch", projectId).Do(&envs); err != nil {
		log.Fatal(errorUnknownError)
	}
	return envs
}
