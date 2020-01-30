package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type EnvironmentVariables struct {
	Results []EnvironmentVariable `json:"results"`
}

func (e EnvironmentVariables) GetEnvironmentVariableByKey(key string) EnvironmentVariable {
	for _, v := range e.Results {
		if v.Key == key {
			return v
		}
	}

	return EnvironmentVariable{}
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

func CreateProjectEnvironmentVariable(environmentVariable EnvironmentVariable, projectId string) EnvironmentVariable {
	ev := EnvironmentVariable{}

	if projectId == "" {
		return ev
	}

	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(environmentVariable)

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/env", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return ev
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ev)

	return ev
}

func DeleteProjectEnvironmentVariable(environmentVariableId string, projectId string) {
	if environmentVariableId == "" || projectId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/env/"+environmentVariableId, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, _ := client.Do(req)

	CheckHTTPResponse(resp)
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

func CreateEnvironmentEnvironmentVariable(environmentVariable EnvironmentVariable, projectId string, branchName string) EnvironmentVariable {
	ev := EnvironmentVariable{}

	if projectId == "" || branchName == "" {
		return ev
	}

	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(environmentVariable)

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/branch/"+branchName+"/env", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return ev
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ev)

	return ev
}

func DeleteEnvironmentEnvironmentVariable(environmentVariableId string, projectId string, branchName string) {
	if environmentVariableId == "" || projectId == "" || branchName == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/branch/"+branchName+"/env/"+environmentVariableId, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, _ := client.Do(req)

	CheckHTTPResponse(resp)
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

func CreateApplicationEnvironmentVariable(environmentVariable EnvironmentVariable, projectId string, repositoryId string,
	environmentId string, applicationId string) EnvironmentVariable {

	ev := EnvironmentVariable{}

	if projectId == "" || repositoryId == "" || environmentId == "" || applicationId == "" {
		return ev
	}

	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(environmentVariable)

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/repository/"+repositoryId+"/environment/"+
		environmentId+"/application/"+applicationId+"/env", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return ev
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ev)

	return ev
}

func DeleteApplicationEnvironmentVariable(environmentVariableId string, projectId string, repositoryId string,
	environmentId string, applicationId string) {

	if projectId == "" || repositoryId == "" || environmentId == "" || applicationId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/repository/"+repositoryId+"/environment/"+
		environmentId+"/application/"+applicationId+"/env/"+environmentVariableId, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, _ := client.Do(req)

	CheckHTTPResponse(resp)
}
