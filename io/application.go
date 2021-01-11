package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Applications struct {
	Results []Application `json:"results"`
}

type Application struct {
	Id             string           `json:"id"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Name           string           `json:"name"`
	Cpu            string           `json:"cpu"`
	RamInMib       int              `json:"ram_in_mib"`
	Status         DeploymentStatus `json:"status"`
	ConnectionURI  string           `json:"connection_uri"`
	TotalDatabases *int             `json:"total_databases"`
	TotalBrokers   *int             `json:"total_brokers"`
	Databases      []Service        `json:"databases"`
	Brokers        []Service        `json:"brokers"`
	Repository     Repository       `json:"repository"`
}

func (a *Application) Ram() string {
	ramInBytes := a.RamInMib * 1024 * 1024

	const unit = 1024
	if ramInBytes < unit {
		return fmt.Sprintf("%d B", ramInBytes)
	}

	div, exp := int64(unit), 0
	for n := ramInBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.0f %cB", float64(ramInBytes)/float64(div), "KMGTPE"[exp])
}

func GetApplicationByName(projectId string, environmentId string, name string, withDetails bool) Application {
	app := Application{}

	if projectId == "" || environmentId == "" || name == "" {
		return app
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL + "/project/" + projectId +
		"/environment/" + environmentId + "/application/name/" + name + "?details=" + strconv.FormatBool(withDetails),  nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return app
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &app)

	return app
}

func (a *Application) GetDatabaseNames() []string {
	var names []string

	for _, x := range a.Databases {
		names = append(names, x.Name)
	}

	return names
}

func (a *Application) GetBrokerNames() []string {
	var names []string

	for _, x := range a.Brokers {
		names = append(names, x.Name)
	}

	return names
}

func ListApplications(projectId string, environmentId string) Applications {
	apps := Applications{}

	if projectId == "" || environmentId == "" {
		return apps
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/project/"+projectId+"/environment/"+environmentId+"/application", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return apps
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &apps)

	return apps
}
