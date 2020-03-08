package cmd

import (
	"fmt"
	"qovery.go/api"
	"qovery.go/util"
	"strconv"
	"strings"
)

func ShowEnvironmentVariablesByProjectName(projectName string, showCredentials bool) {
	projectId := api.GetProjectByName(projectName).Id
	evs := api.ListProjectEnvironmentVariables(projectId)
	ShowEnvironmentVariables(evs.Results, showCredentials)
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

func ShowEnvironmentVariablesByBranchName(projectName string, branchName string, showCredentials bool) {
	projectId := api.GetProjectByName(projectName).Id

	var evs []api.EnvironmentVariable

	for _, ev := range getStaticBuiltInEnvironmentVariables(branchName) {
		evs = append(evs, ev)
	}

	for _, ev := range api.ListEnvironmentEnvironmentVariables(projectId, branchName).Results {
		evs = append(evs, ev)
	}

	ShowEnvironmentVariables(evs, showCredentials)
}

func ShowEnvironmentVariablesByApplicationName(projectName string, branchName string, showCredentials bool) {
	ShowEnvironmentVariables(ListEnvironmentVariables(projectName, branchName), showCredentials)
}

func ListEnvironmentVariables(projectName string, branchName string) []api.EnvironmentVariable {
	projectId := api.GetProjectByName(projectName).Id
	repositoryId := api.GetRepositoryByCurrentRemoteURL(projectId).Id
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

func ShowEnvironmentVariables(environmentVariables []api.EnvironmentVariable, showCredentials bool) {
	table := util.GetTable()
	table.SetHeader([]string{"scope", "key", "value"})

	for _, ev := range environmentVariables {
		lowerCaseKey := strings.ToLower(ev.Key)
		if !showCredentials && (strings.Contains(lowerCaseKey, "username") || strings.Contains(lowerCaseKey, "password") ||
			strings.Contains(lowerCaseKey, "fqdn") || strings.Contains(lowerCaseKey, "host") || strings.Contains(lowerCaseKey, "port") ||
			strings.Contains(lowerCaseKey, "uri")) {
			table.Append([]string{ev.Scope, ev.Key, "<hidden>"})
		} else {
			table.Append([]string{ev.Scope, ev.Key, ev.Value})
		}
	}

	table.Render()
}
