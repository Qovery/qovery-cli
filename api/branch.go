package api

import (
	"encoding/json"
	"io/ioutil"
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

	req, _ := http.NewRequest("GET", RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/branch", nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	r := AggregatedEnvironments{}

	if err != nil {
		return r
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}
