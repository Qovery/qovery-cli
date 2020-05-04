package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type GitAccessStatus struct {
	HasAccess       bool   `json:"has_access"`
	Message         string `json:"message"`
	GitURL          string `json:"git_url"`
	SanitizedGitURL string `json:"sanitized_git_url"`
}

func GitCheck(gitURL string) GitAccessStatus {
	gas := GitAccessStatus{}

	if gitURL == "" {
		return gas
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/git/access/check?url="+gitURL, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return gas
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &gas)

	return gas
}
