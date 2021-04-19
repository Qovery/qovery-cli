package io

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func Deploy(projectId string, environmentId string, applicationId string, commitId string) {
	if projectId == "" || environmentId == "" || applicationId == "" || commitId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/application/"+applicationId+"/commit/"+commitId+"/deploy", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return
	}

	if resp != nil && resp.StatusCode == http.StatusBadRequest {
		fmt.Println("Could not deploy application with commit " + commitId)
		fmt.Println("Are you sure you entered a valid commit sha?")
		os.Exit(1)
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func AdminDeploy(clusterId string, dryRunDisabled bool){
	authToken,_ := GetTokens()
	var req *http.Request
	var err error

	if !dryRunDisabled {
		body := bytes.NewBuffer([]byte( `{"metadata": {"dry_run_deploy": true}}`))
		req, err  = http.NewRequest(http.MethodPost, RootURL + "/infrastructure/init/" + clusterId, body )
	} else {
		req, err  = http.NewRequest(http.MethodPost, RootURL + "/infrastructure/init/" + clusterId, nil )
	}

	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer " + strings.TrimSpace(authToken))


	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(res.Status, "200") {
		result, _ := ioutil.ReadAll(res.Body)
		log.Errorf("Could not deploy cluster : %s. %s", res.Status, string(result) )
	} else {
		fmt.Println("Cluster " + clusterId + " deploying.")
	}

}
