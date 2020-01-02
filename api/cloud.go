package api

import (
	"log"
	"net/http"
)

type CloudProviders struct {
	Results []CloudProvider `json:"results"`
}

type CloudProvider struct {
	ObjectType string                `json:"object_type"`
	Id         string                `json:"id"`
	Name       string                `json:"name"`
	Regions    []CloudProviderRegion `json:"regions"`
}

type CloudProviderRegion struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
	FullName   string `json:"full_name"`
}

func ListCloudProviders() CloudProviders {
	CheckAuthenticationOrQuitWithMessage()
	var providers CloudProviders
	if err := NewRequest(http.MethodGet, "/cloud").Do(&providers); err != nil {
		log.Fatal(errorUnknownError)
	}
	return providers
}
