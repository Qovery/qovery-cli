package api

import (
	"log"
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
	var projects Projects
	if err := NewRequest(http.MethodGet, "/project").Do(&projects); err != nil {
		log.Fatalf(errorUnknownError)
	}
	return projects
}

func CreateProject(project Project) Project {
	CheckAuthenticationOrQuitWithMessage()
	var responseProject Project
	if err := NewRequest(http.MethodPost, "/project").SetJsonBody(&project).Do(&responseProject); err != nil {
		log.Fatal(errorUnknownError)
	}
	return responseProject
}
