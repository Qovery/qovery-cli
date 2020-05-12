package cmd

import (
	"fmt"
	"qovery.go/io"
	"strconv"
	"strings"
)

func ShowEnvironmentVariablesByProjectName(projectName string, showCredentials bool) {
	projectId := io.GetProjectByName(projectName).Id
	evs := io.ListProjectEnvironmentVariables(projectId)
	ShowEnvironmentVariables(evs.Results, showCredentials)
}

func getStaticBuiltInEnvironmentVariables(branchName string) []io.EnvironmentVariable {
	isProduction := false
	if branchName == "master" {
		isProduction = true
	}

	return []io.EnvironmentVariable{
		{Scope: "BUILT_IN", Key: "QOVERY_BRANCH_NAME", Value: branchName, KeyValue: fmt.Sprintf("QOVERY_BRANCH_NAME=%s", branchName)},
		{Scope: "BUILT_IN", Key: "QOVERY_IS_PRODUCTION", Value: strconv.FormatBool(isProduction),
			KeyValue: fmt.Sprintf("QOVERY_IS_PRODUCTION=%s", strconv.FormatBool(isProduction))},
	}
}

func ShowEnvironmentVariablesByBranchName(projectName string, branchName string, showCredentials bool) {
	projectId := io.GetProjectByName(projectName).Id

	var evs []io.EnvironmentVariable

	evs = append(evs, getStaticBuiltInEnvironmentVariables(branchName)...)
	environmentId := io.GetEnvironmentByName(projectId, branchName).Id
	evs = append(evs, io.ListEnvironmentEnvironmentVariables(projectId, environmentId).Results...)

	ShowEnvironmentVariables(evs, showCredentials)
}

func ShowEnvironmentVariablesByApplicationName(projectName string, branchName string, applicationName string, showCredentials bool) {
	ShowEnvironmentVariables(ListEnvironmentVariables(projectName, branchName, applicationName), showCredentials)
}

func ListEnvironmentVariables(projectName string, branchName string, applicationName string) []io.EnvironmentVariable {
	projectId := io.GetProjectByName(projectName).Id
	environment := io.GetEnvironmentByName(projectId, branchName)
	application := io.GetApplicationByName(projectId, environment.Id, applicationName)

	var evs []io.EnvironmentVariable

	evs = append(evs, getStaticBuiltInEnvironmentVariables(branchName)...)
	evs = append(evs, io.ListApplicationEnvironmentVariables(projectId, environment.Id, application.Id).Results...)

	return evs
}

func ShowEnvironmentVariables(environmentVariables []io.EnvironmentVariable, showCredentials bool) {
	table := io.GetTable()
	table.SetHeader([]string{"scope", "key", "value"})

	for _, ev := range environmentVariables {
		lowerCaseKey := strings.ToLower(ev.Key)
		if !showCredentials && (strings.Contains(lowerCaseKey, "username") || strings.Contains(lowerCaseKey, "password") ||
			strings.Contains(lowerCaseKey, "fqdn") || strings.Contains(lowerCaseKey, "host") || strings.Contains(lowerCaseKey, "port") ||
			strings.Contains(lowerCaseKey, "uri") || strings.Contains(lowerCaseKey, "key")) {
			table.Append([]string{ev.Scope, ev.Key, "<hidden>"})
		} else {
			table.Append([]string{ev.Scope, ev.Key, ev.Value})
		}
	}

	table.Render()
}
