package io

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net/http"
	"os"
)

type Routers struct {
	Results []Router `json:"results"`
}

type Router struct {
	Name          string       `json:"name"`
	ConnectionURI string       `json:"connection_uri"`
	CustomDomain  CustomDomain `json:"custom_domain"`
	DeletedAt     string       `json:"deleted_at"`
}

type DomainStatus string

const (
	Verifying DomainStatus = "VERIFYING"
	Verified  DomainStatus = "VERIFIED"
)

type Status struct {
	Status DomainStatus
}

type CustomDomain struct {
	Domain       string `json:"provided_domain"`
	TargetDomain string `json:"target_domain"`
	Status       Status `json:"status"`
}

func (c *CustomDomain) GetDomain() string {
	if c.Domain == "" {
		return color.RedString("unknown")
	}

	return c.Domain
}

func (c *CustomDomain) GetTargetDomain() string {
	if c.TargetDomain == "" {
		return color.RedString("unknown")
	}

	return c.TargetDomain
}

func ListRouters(projectId string, environmentId string) Routers {
	routers := Routers{}

	if projectId == "" || environmentId == "" {
		return routers
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/router", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return routers
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &routers)

	filteredRouters := Routers{}
	for _, p := range routers.Results {
		if p.DeletedAt == "" {
			filteredRouters.Results = append(filteredRouters.Results, p)
		}
	}

	return filteredRouters
}
