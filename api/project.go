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
	ObjectType          string              `json:"object_type"`
	Id                  string              `json:"id"`
	CreatedAt           string              `json:"created_at"`
	UpdatedAt           string              `json:"updated_at"`
	Name                string              `json:"name"`
	CloudProviderRegion CloudProviderRegion `json:"cloud_provider_region"`
}

func GetProjectByName(name string) Project {
	for _, p := range ListProjects().Results {
		if p.Name == name {
			return p
		}
	}

	return Project{}
}

func ListProjects() Projects {
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	p := Projects{}

	if err != nil {
		return p
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &p)

	return p
}
