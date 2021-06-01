package io

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
)

type ExecInfos struct {
	GitId string
	ProjectId string
	EnvId string
	AppId string
	ClusterId string
	OrgaId string
}

func GetInfos(execId string) ExecInfos {
	envId := getEnvironmentId(execId)
	fmt.Println(envId)

	return ExecInfos{}
}

func getEnvironmentId(execId string) string {
	authToken, _ := GetTokens()
	var req *http.Request
	var err error

	body := bytes.NewBuffer([]byte(fmt.Sprintf(`{"type": "FINGERPRINT", "value": "%s"}`, execId)))
	//req, err = http.NewRequest(http.MethodPost, RootURL+"/admin/deployment", body)
	req, err = http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/admin/deployment", body)

	if err != nil {
		log.Fatal(err)
		return ""
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(authToken))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
		return ""
	}

	if !strings.Contains(res.Status, "200") {
		result, _ := ioutil.ReadAll(res.Body)
		log.Errorf(string(result))
		return ""
	} else {
		result, _ := ioutil.ReadAll(res.Body)
		return string(result)
	}
}
