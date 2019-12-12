package api

import (
	"encoding/json"
	"io/ioutil"
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

func ListApplications(projectId string, branchId string) Applications {
	req, _ := http.NewRequest("GET", RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/branch/"+branchId+"/application", nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	apps := Applications{}

	if err != nil {
		return apps
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &apps)

	return apps
}
