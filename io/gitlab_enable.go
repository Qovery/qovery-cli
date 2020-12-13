package io

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

type GitlabEnable struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

func EnableGitlabWebhooks(gitlabEnable GitlabEnable) {
	token := GetAuthorizationToken()
	client := &http.Client{}

	url := RootURL + "/hook/gitlab/enable"
	body, err := json.Marshal(gitlabEnable)
	CheckIfError(err)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerAuthorization, headerValueBearer+token)

	res, err := client.Do(req)

	if err != nil || res.StatusCode != 204 {
		println("Could not enable Qovery in " + gitlabEnable.Group + "/" + gitlabEnable.Name)
		os.Exit(1)
	}

	println("Enabled Qovery in " + gitlabEnable.Group + "/" + gitlabEnable.Name)
}
