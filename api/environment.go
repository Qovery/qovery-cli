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
	BranchId          string        `json:"branch_id"`
	Status            Status        `json:"status"`
	ConnectionURIs    []string      `json:"connection_uris"`
	TotalApplications *int          `json:"total_applications"`
	TotalDatabases    *int          `json:"total_databases"`
	TotalBrokers      *int          `json:"total_brokers"`
	TotalStorage      *int          `json:"total_storage"`
	Environments      []Environment `json:"environments"`
}

type Environment struct {
	Id          string      `json:"id"`
	BranchId    string      `json:"branch_id"`
	CommitId    string      `json:"commit_id"`
	Application Application `json:"application"`
}

func GetBranchByName(projectId string, name string) AggregatedEnvironment {
	for _, v := range ListBranches(projectId).Results {
		if v.BranchId == name {
			return v
		}
	}

	return AggregatedEnvironment{}
}

func ListBranches(projectId string) AggregatedEnvironments {
	r := AggregatedEnvironments{}

	if projectId == "" {
		return r
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/branch", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return r
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}
