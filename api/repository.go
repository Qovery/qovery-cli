package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"qovery.go/util"
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

func GetRepositoryByCurrentRemoteURL(projectId string) Repository {
	for _, url := range util.ListRemoteURLs() {
		r := GetRepositoryByRemoteURL(projectId, url)
		if r.Id != "" {
			return r
		}
	}

	return Repository{}
}

func cleanRepositoryURL(url string) string {
	if !strings.HasSuffix(url, ".git") {
		url = url + ".git"
	}

	re := regexp.MustCompile("https:\\/\\/(.*@)")
	match := re.FindStringSubmatch(url)
	if len(match) == 2 {
		url = strings.Replace(url, match[1], "", 1)
	}
	return url
}

func GetRepositoryByRemoteURL(projectId string, url string) Repository {
	url = cleanRepositoryURL(url)
	for _, v := range ListRepositories(projectId).Results {
		if v.URL == url {
			return v
		}
	}

	return Repository{}
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

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/repository", nil)
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
