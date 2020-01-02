package api

import (
	"bytes"
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

func GetRepositoryByName(projectId string, name string) *Repository {
	for _, v := range ListRepositories(projectId).Results {
		if v.Name == name {
			return &v
		}
	}

	return nil
}

func ListRepositories(projectId string) Repositories {
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/repository", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	r := Repositories{}

	if err != nil {
		return r
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}

func CreateRepository(projectId string, repository Repository) Repository {
	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(repository)

	req, _ := http.NewRequest(MethodPost, RootURL+"/user/"+GetAccountId()+"/project/"+projectId+"/repository", b)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	r := Repository{}

	if err != nil {
		return r
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}
