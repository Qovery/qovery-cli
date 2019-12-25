package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Projects struct {
	Results []Project `json:"results"`
}

type Project struct {
	ObjectType          string              `json:"object_type"`
	Id                  string              `json:"id"`
	CreatedAt           string              `json:"created_at"`
	UpdatedAt           string              `json:"updated_at"`
	Name                string              `json:"name"`
	CloudProviderRegion CloudProviderRegion `json:"cloud_provider_region"`
}

func GetProjectByName(name string) *Project {
	for _, v := range ListProjects().Results {
		if v.Name == name {
			return &v
		}
	}

	return nil
}

func ListProjects() Projects {
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest("GET", RootURL+"/user/"+GetAccountId()+"/project", nil)
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

func CreateProject(project Project) Project {
	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(project)

	req, _ := http.NewRequest("POST", RootURL+"/user/"+GetAccountId()+"/project", b)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	p := Project{}

	if err != nil {
		return p
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &p)

	return p
}
