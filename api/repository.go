package api

import (
	"log"
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
	var repo Repositories
	if err := NewRequest(http.MethodGet, "/user/%s/project/%s/repository", GetAccountId(), projectId).Do(&repo); err != nil {
		log.Fatal(errorUnknownError)
	}
	return repo
}

func CreateRepository(projectId string, repository Repository) Repository {
	CheckAuthenticationOrQuitWithMessage()
	var r Repository
	if err := NewRequest(http.MethodPost, "/user/%s/project/%s/repository", GetAccountId(), projectId).
		SetJsonBody(repository).Do(&r); err != nil {

		log.Fatalf(errorUnknownError)
	}
	return r
}
