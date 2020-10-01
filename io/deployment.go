package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type Deployments struct {
	Results []Deployment `json:"results"`
}

type Deployment struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func ListDeployments(projectId string, environmentId string) Deployments {
	r := Deployments{}

	if projectId == "" || environmentId == "" {
		return r
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/deployment", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return r
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}
