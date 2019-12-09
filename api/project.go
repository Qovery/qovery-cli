package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Projects struct {
	Results []Project `json:"results"`
}

type Project struct {
	ObjectType          string `json:"object_type"`
	Id                  string `json:"id"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	Name                string `json:"name"`
	CloudProviderRegion struct {
		ObjectType string `json:"object_type"`
		Id         string `json:"id"`
	}
}

func ListProjects() Projects {
	account := GetAccount()

	req, _ := http.NewRequest("GET", RootURL+"/user/"+account.Id+"/project", nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	p := Projects{}

	if err != nil {
		return p
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &p)

	return p
}
