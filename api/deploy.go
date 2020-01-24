package api

import (
	"net/http"
)

func Deploy(projectId string, branchName string, applicationId string, commitId string) {
	if projectId == "" || branchName == "" || applicationId == "" || commitId == "" {
		return
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodPost, RootURL+"/project/"+projectId+"/branch/"+branchName+"/application/"+applicationId+"/commit/"+commitId+"/deploy", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	if err != nil {
		return
	}
}
