package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"qovery.go/api"
)

func ShowEnvironmentVariablesByProjectName(projectName string) {
	projectId := api.GetProjectByName(projectName).Id
	evs := api.ListProjectEnvironmentVariables(projectId)
	ShowEnvironmentVariables(evs.Results)
}

func ShowEnvironmentVariablesByBranchName(projectName string, branchName string) {
	projectId := api.GetProjectByName(projectName).Id
	evs := api.ListEnvironmentEnvironmentVariables(projectId, branchName)
	ShowEnvironmentVariables(evs.Results)
}

func ShowEnvironmentVariablesByApplicationName(projectName string, branchName string, applicationName string) {
	projectId := api.GetProjectByName(projectName).Id
	repositoryId := api.GetRepositoryByName(projectId, applicationName).Id
	environment := api.GetEnvironmentByBranchId(projectId, repositoryId, branchName)
	evs := api.ListApplicationEnvironmentVariables(projectId, repositoryId, environment.Id, environment.Application.Id)
	ShowEnvironmentVariables(evs.Results)
}

func ShowEnvironmentVariables(environmentVariables []api.EnvironmentVariable) {
	output := []string{"scope | key | value"}

	if environmentVariables == nil || len(environmentVariables) == 0 {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	for _, ev := range environmentVariables {
		output = append(output, ev.Scope+"|"+ev.Key+"|"+ev.Value)
	}

	fmt.Println(columnize.SimpleFormat(output))

}
