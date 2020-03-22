package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"qovery.go/util"
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
	var projects []Project

	for _, p := range ListProjects().Results {
		if p.Name == name {
			projects = append(projects, p)
		}
	}

	if len(projects) == 0 {
		return Project{}
	} else if len(projects) == 1 {
		return projects[0]
	}

	remoteURLs := util.ListRemoteURLs()

	// take the right project from matching local and distant remote URL
	for _, p := range projects {
		// TODO improve
		for _, r := range ListRepositories(p.Id).Results {
			for _, url := range remoteURLs {
				if r.URL == url {
					return p
				}
			}
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

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p := Projects{}

	if err != nil {
		return p
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &p)

	return p
}
