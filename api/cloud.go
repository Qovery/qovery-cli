package api

import (
	"encoding/json"
	"io/ioutil"
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
	req, _ := http.NewRequest("GET", RootURL+"/cloud", nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	c := CloudProviders{}

	if err != nil {
		return c
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &c)

	return c
}
