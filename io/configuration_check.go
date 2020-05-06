package io

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type ConfigurationCheckRequest struct {
	QoveryYMLContent  string `json:"qovery_yml_content"`
	DockerfileContent string `json:"dockerfile_content"`
}

type ConfigurationCheckResponse struct {
	Valid  bool                      `json:"valid"`
	Errors []ConfigurationCheckError `json:"errors"`
}

type ConfigurationCheckError struct {
	LineNumber int    `json:"line_number"`
	Reason     string `json:"reason"`
	Hint       string `json:"hint"`
}

func DoCheckConfiguration(request ConfigurationCheckRequest) ConfigurationCheckResponse {
	ccr := ConfigurationCheckResponse{}

	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(request)

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/configuration/check", b)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return ccr
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &ccr)

	return ccr
}
