package io

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

	if err != nil {
		return evs
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	if err != nil {
		return ev
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	err := CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func ListEnvironmentEnvironmentVariables(projectId string, environmentId string) EnvironmentVariables {
	evs := EnvironmentVariables{}

	if projectId == "" || environmentId == "" {
		return evs
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/env", nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return evs
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &evs)

	return evs
}

func CreateEnvironmentEnvironmentVariable(environmentVariable EnvironmentVariable, projectId string, environmentId string) EnvironmentVariable {
	ev := EnvironmentVariable{}

	if projectId == "" || environmentId == "" {
		return ev
	}

	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(environmentVariable)

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/env", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return ev
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ev)

	return ev
}

func DeleteEnvironmentEnvironmentVariable(environmentVariableId string, projectId string, environmentId string) {
	if environmentVariableId == "" || projectId == "" || environmentId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/env/"+environmentVariableId, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, _ := client.Do(req)

	err := CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func ListApplicationEnvironmentVariables(projectId string, environmentId string, applicationId string) EnvironmentVariables {
	evs := EnvironmentVariables{}

	if projectId == "" || environmentId == "" || applicationId == "" {
		return evs
	}

	CheckAuthenticationOrQuitWithMessage()

	url := RootURL + "/project/" + projectId + "/environment/" + environmentId + "/application/" + applicationId + "/env"
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return evs
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &evs)

	return evs
}

func CreateApplicationEnvironmentVariable(environmentVariable EnvironmentVariable, projectId string, environmentId string,
	applicationId string) EnvironmentVariable {

	ev := EnvironmentVariable{}

	if projectId == "" || environmentId == "" || applicationId == "" {
		return ev
	}

	CheckAuthenticationOrQuitWithMessage()

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(environmentVariable)

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/environment/"+
		environmentId+"/application/"+applicationId+"/env", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return ev
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ev)

	return ev
}

func DeleteApplicationEnvironmentVariable(environmentVariableId string, projectId string,
	environmentId string, applicationId string) {

	if projectId == "" || environmentId == "" || applicationId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodDelete, RootURL+"/project/"+projectId+"/environment/"+
		environmentId+"/application/"+applicationId+"/env/"+environmentVariableId, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, _ := client.Do(req)

	err := CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
