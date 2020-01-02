package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Repositories struct {
	Results []Repository `json:"results"`
}

type Repository struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Name       string `json:"name"`
	URL        string `json:"url"`
}

func GetRepositoryByName(projectId string, name string) Repository {
	for _, v := range ListRepositories(projectId).Results {
		if v.Name == name {
			return v
		}
	}

	return Repository{}
}

func ListRepositories(projectId string) Repositories {
	r := Repositories{}

	if projectId == "" {
		return r
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/repository", nil)
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
