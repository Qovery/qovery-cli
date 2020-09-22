package io

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

type Logs struct {
	Results []Log `json:"results"`
}

type Log struct {
	Id        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

func ListApplicationLogs(lastLines int, follow bool, projectId string, environmentId string, applicationId string) Logs {
	logs := Logs{}

	if projectId == "" || environmentId == "" || applicationId == "" {
		return logs
	}

	CheckAuthenticationOrQuitWithMessage()

	url := RootURL + "/project/" + projectId + "/environment/" + environmentId + "/application/" + applicationId + "/log"

	req, _ := http.NewRequest(http.MethodGet, url, nil)

	req.URL.Query().Set("tail", strconv.Itoa(lastLines))
	req.URL.Query().Set("follow", strconv.FormatBool(follow))

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("accept", "application/stream+json")

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return logs
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	d := json.NewDecoder(resp.Body)
	for {
		var v Log
		if err := d.Decode(&v); err == io.EOF {
			break
		} else if err != nil {
			// handle error
		}
		fmt.Print(v.Message)
	}

	return logs
}
