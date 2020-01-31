package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"qovery.go/api"
	"strconv"
)

func ShowEnvironmentVariablesByProjectName(projectName string) {
	projectId := api.GetProjectByName(projectName).Id
	evs := api.ListProjectEnvironmentVariables(projectId)
	ShowEnvironmentVariables(evs.Results)
}

func getStaticBuiltInEnvironmentVariables(branchName string) []api.EnvironmentVariable {
	isProduction := false
	if branchName == "master" {
		isProduction = true
	}

	return []api.EnvironmentVariable{
		{Scope: "BUILT_IN", Key: "QOVERY_JSON_B64", Value: "<base64>", KeyValue: "QOVERY_JSON_B64=<base64>"},
		{Scope: "BUILT_IN", Key: "QOVERY_BRANCH_NAME", Value: branchName, KeyValue: fmt.Sprintf("QOVERY_BRANCH_NAME=%s", branchName)},
		{Scope: "BUILT_IN", Key: "QOVERY_IS_PRODUCTION", Value: strconv.FormatBool(isProduction),
			KeyValue: fmt.Sprintf("QOVERY_IS_PRODUCTION=%s", strconv.FormatBool(isProduction))},
	}
}

func ShowEnvironmentVariablesByBranchName(projectName string, branchName string) {
	projectId := api.GetProjectByName(projectName).Id

	var evs []api.EnvironmentVariable

	for _, ev := range getStaticBuiltInEnvironmentVariables(branchName) {
		evs = append(evs, ev)
	}

	for _, ev := range api.ListEnvironmentEnvironmentVariables(projectId, branchName).Results {
		evs = append(evs, ev)
	}

	ShowEnvironmentVariables(evs)
}

func ShowEnvironmentVariablesByApplicationName(projectName string, branchName string, applicationName string) {
	ShowEnvironmentVariables(ListEnvironmentVariables(projectName, branchName, applicationName))
}

func ListEnvironmentVariables(projectName string, branchName string, applicationName string) []api.EnvironmentVariable {
	projectId := api.GetProjectByName(projectName).Id
	repositoryId := api.GetRepositoryByName(projectId, applicationName).Id
	environment := api.GetEnvironmentByBranchId(projectId, repositoryId, branchName)

	var evs []api.EnvironmentVariable

	for _, ev := range getStaticBuiltInEnvironmentVariables(branchName) {
		evs = append(evs, ev)
	}

	for _, ev := range api.ListApplicationEnvironmentVariables(projectId, repositoryId, environment.Id, environment.Application.Id).Results {
		evs = append(evs, ev)
	}

	return evs
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
