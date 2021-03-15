package cmd

import (
	"fmt"
	"github.com/Qovery/qovery-cli/io"
	"strconv"
	"strings"
)

func ShowEnvironmentVariablesByProjectName(organizationName string, projectName string, showCredentials bool, outputEnvironmentVariables bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	evs := io.ListProjectEnvironmentVariables(projectId)

	if outputEnvironmentVariables {
		ShowEnvironmentVariables(evs.Results, showCredentials)
		return
	}

	ShowEnvironmentVariablesWithTableFormat(evs.Results, showCredentials)
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

func ShowEnvironmentVariablesByBranchName(organizationName string, projectName string, branchName string, showCredentials bool, outputEnvironmentVariables bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id

	var evs []io.EnvironmentVariable

	evs = append(evs, getStaticBuiltInEnvironmentVariables(branchName)...)
	environmentId := io.GetEnvironmentByName(projectId, branchName, true).Id
	evs = append(evs, io.ListEnvironmentEnvironmentVariables(projectId, environmentId).Results...)

	if outputEnvironmentVariables {
		ShowEnvironmentVariables(evs, showCredentials)
		return
	}

	ShowEnvironmentVariablesWithTableFormat(evs, showCredentials)
}

func ShowEnvironmentVariablesByApplicationName(organizationName string, projectName string, branchName string, applicationName string, showCredentials bool, outputEnvironmentVariables bool) {
	evs := ListEnvironmentVariables(organizationName, projectName, branchName, applicationName)

	if outputEnvironmentVariables {
		ShowEnvironmentVariables(evs, showCredentials)
		return
	}

	ShowEnvironmentVariablesWithTableFormat(evs, showCredentials)
}

func ListEnvironmentVariables(organizationName string, projectName string, branchName string, applicationName string) []io.EnvironmentVariable {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	application := io.GetApplicationByName(projectId, environment.Id, applicationName, true)

	var evs []io.EnvironmentVariable

	evs = append(evs, getStaticBuiltInEnvironmentVariables(branchName)...)
	evs = append(evs, io.ListApplicationEnvironmentVariables(projectId, environment.Id, application.Id).Results...)

	return evs
}

func ShowEnvironmentVariablesWithTableFormat(environmentVariables []io.EnvironmentVariable, showCredentials bool) {
	table := io.GetTable()
	table.SetHeader([]string{"scope", "key", "value"})

	for _, ev := range environmentVariables {
		if !showCredentials && isSensitive(ev.Key) {
			table.Append([]string{ev.Scope, ev.Key, "<hidden>"})
		} else {
			table.Append([]string{ev.Scope, ev.Key, ev.Value})
		}
	}

	table.Render()
}

func ShowEnvironmentVariables(environmentVariables []io.EnvironmentVariable, showCredentials bool) {
	for _, ev := range environmentVariables {
		if (!showCredentials && !isSensitive(ev.Key)) || showCredentials {
			fmt.Println(ev.KeyValue)
		}
	}
}

func isSensitive(key string) bool {
	lowerCaseKey := strings.ToLower(key)
	return strings.Contains(lowerCaseKey, "username") || strings.Contains(lowerCaseKey, "password") ||
		strings.Contains(lowerCaseKey, "fqdn") || strings.Contains(lowerCaseKey, "host") || strings.Contains(lowerCaseKey, "port") ||
		strings.Contains(lowerCaseKey, "uri") || strings.Contains(lowerCaseKey, "key")
}
