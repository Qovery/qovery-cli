package io

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type DeploymentStatuses struct {
	Results []DeploymentStatus `json:"results"`
}

type DeploymentStatus struct {
	Id             string         `json:"id"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	Scope          string         `json:"scope"`
	Level          string         `json:"level"`
	Message        string         `json:"message"`
	StatusForHuman StatusForHuman `json:"status_for_human"`
}

type StatusForHuman struct {
	Long  string `json:"long"`
	Short string `json:"short"`
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
	return s.IsDeployed() || s.IsPaused() || s.IsDeleted()
}

func (s *DeploymentStatus) IsInProgress() bool {
	return s.IsDeploymentInProgress() || s.IsPauseInProgress() || s.IsDeleteInProgress()
}

func (s *DeploymentStatus) IsNotOk() bool {
	return s.IsStartError() || s.IsPauseError() || s.IsDeleteError()
}

func (s *DeploymentStatus) IsDeployed() bool {
	return s.Status == "DEPLOYED"
}

func (s *DeploymentStatus) IsPaused() bool {
	return s.Status == "PAUSED"
}

func (s *DeploymentStatus) IsDeleted() bool {
	return s.Status == "DELETED"
}

func (s *DeploymentStatus) IsDeploymentInProgress() bool {
	return s.Status == "DEPLOYMENT_IN_PROGRESS"
}

func (s *DeploymentStatus) IsPauseInProgress() bool {
	return s.Status == "PAUSE_IN_PROGRESS"
}

func (s *DeploymentStatus) IsDeleteInProgress() bool {
	return s.Status == "DELETE_IN_PROGRESS"
}

func (s *DeploymentStatus) IsStartError() bool {
	return s.Status == "DEPLOYMENT_ERROR"
}

func (s *DeploymentStatus) IsPauseError() bool {
	return s.Status == "PAUSE_ERROR"
}

func (s *DeploymentStatus) IsDeleteError() bool {
	return s.Status == "DELETE_ERROR"
}

func (s *DeploymentStatus) GetColoredStatus() string {
	if s.Status == "" {
		return color.RedString("unknown")
	}

	if s.IsOk() {
		return color.GreenString(s.StatusForHuman.Long)
	} else if s.IsNotOk() {
		return color.RedString(s.StatusForHuman.Long)
	}

	// running and other states are yellow
	return color.YellowString(s.StatusForHuman.Long)
}

func (s *DeploymentStatus) GetColoredMessage() string {
	if s.IsLevelError() {
		return color.RedString(s.Message)
	} else if s.IsLevelWarn() {
		return color.YellowString(s.Message)
	}

	return s.Message
}

func (s *DeploymentStatus) GetColoredLevel() string {
	if s.IsLevelError() {
		return color.RedString(s.Level)
	} else if s.IsLevelWarn() {
		return color.YellowString(s.Level)
	}

	return color.GreenString(s.Level)
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
