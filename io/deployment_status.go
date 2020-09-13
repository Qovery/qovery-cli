package io

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type DeploymentStatuses struct {
	Results []DeploymentStatus `json:"results"`
}

type DeploymentStatus struct {
	Id      string `json:"id"`
	Status  string `json:"status"`
	Scope   string `json:"scope"`
	Step    string `json:"step"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (s *DeploymentStatus) GetKind() string {
	if s.Status == "" {
		return "unknown"
	}

	return s.Status
}

func (s *DeploymentStatus) IsLevelInfo() bool {
	return s.Level == "INFO"
}

func (s *DeploymentStatus) IsLevelWarn() bool {
	return s.Level == "WARN"
}

func (s *DeploymentStatus) IsLevelError() bool {
	return s.Level == "ERROR"
}

func (s *DeploymentStatus) IsLevelDebug() bool {
	return s.Level == "INFO"
}

func (s *DeploymentStatus) IsOk() bool {
	return s.IsTerminated() || s.IsRunning()
}

func (s *DeploymentStatus) IsTerminated() bool {
	return s.Status == "TERMINATED"
}

func (s *DeploymentStatus) IsRunning() bool {
	return s.Status == "RUNNING"
}

func (s *DeploymentStatus) IsTerminatedWithError() bool {
	return s.Status == "TERMINATED_WITH_ERROR"
}

func (s *DeploymentStatus) GetColoredStatus() string {
	if s.Status == "" {
		return color.RedString("unknown")
	}

	if s.IsTerminated() {
		return color.GreenString(strings.ToLower("running"))
	} else if s.IsTerminatedWithError() {
		return color.RedString(strings.ToLower("an error occurred"))
	}

	// running and other states are yellow
	return color.YellowString(strings.ToLower("deployment in progress"))
}

func (s *DeploymentStatus) GetColoredMessage() string {
	if s.IsLevelError() {
		return color.RedString(s.Message)
	} else if s.IsLevelWarn() {
		return color.YellowString(s.Message)
	}

	return s.Message
}

func ListDeploymentStatuses(projectId string, environmentId string, deploymentId string) DeploymentStatuses {
	r := DeploymentStatuses{}

	if projectId == "" || environmentId == "" || deploymentId == "" {
		return r
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/deployment/"+deploymentId+"/status", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return r
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &r)

	return r
}
