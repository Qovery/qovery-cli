package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type EnvironmentVariables struct {
	Results []EnvironmentVariable `json:"results"`
}

type EnvironmentVariable struct {
	Id       string `json:"id"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	Scope    string `json:"scope"`
	KeyValue string `json:"key_value"`
}

func ListProjectEnvironmentVariables(projectId string) EnvironmentVariables {
	evs := EnvironmentVariables{}

	if projectId == "" {
		return evs
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/env", nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return evs
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &evs)

	return evs
}

func ListEnvironmentEnvironmentVariables(projectId string, branchName string) EnvironmentVariables {
	evs := EnvironmentVariables{}

	if projectId == "" || branchName == "" {
		return evs
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/branch/"+branchName+"/env", nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return evs
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &evs)

	return evs
}

func ListApplicationEnvironmentVariables(projectId string, repositoryId string, environmentId string, applicationId string) EnvironmentVariables {
	evs := EnvironmentVariables{}

	if projectId == "" || repositoryId == "" || environmentId == "" || applicationId == "" {
		return evs
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/repository/"+repositoryId+"/environment/"+
		environmentId+"/application/"+applicationId+"/env", nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return evs
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &evs)

	return evs
}
