package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var id string
var organizationName string
var projectName string
var environmentName string
var watchFlag bool
var markdownFlag bool
var jiraFlag bool
var jsonFlag bool
var servicesJson string

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List services",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		orgId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		apps, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		jobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if markdownFlag {
			markdown := getMarkdownOutput(*client, orgId, projectId, envId, apps.GetResults(), containers.GetResults(), jobs.GetResults(), databases.GetResults())
			fmt.Print(markdown)
			return
		}

		if jiraFlag {
			jira := getJiraOutput(*client, orgId, projectId, envId, apps.GetResults(), containers.GetResults(), jobs.GetResults(), databases.GetResults())
			fmt.Print(jira)
			return
		}

		if jsonFlag {
			j := getServiceJsonOutput(*statuses, apps.GetResults(), containers.GetResults(), jobs.GetResults(), databases.GetResults(), helms.GetResults())
			fmt.Print(j)
			return
		}

		var data [][]string

		for _, app := range apps.GetResults() {
			data = append(data, []string{app.GetName(), "Application", utils.FindStatusTextWithColor(statuses.GetApplications(), app.Id)})
		}

		for _, container := range containers.GetResults() {
			data = append(data, []string{container.Name, "Container", utils.FindStatusTextWithColor(statuses.GetContainers(), container.Id)})
		}

		for _, job := range jobs.GetResults() {
			jobType := "Lifecycle"
			if job.CronJobResponse != nil {
				jobType = "Cronjob"
			}

			data = append(data, []string{utils.GetJobName(&job), jobType, utils.FindStatusTextWithColor(statuses.GetJobs(), utils.GetJobId(&job))})
		}

		for _, database := range databases.GetResults() {
			data = append(data, []string{database.Name, "Database", utils.FindStatusTextWithColor(statuses.GetDatabases(), database.Id)})
		}

		for _, helm := range helms.GetResults() {
			data = append(data, []string{helm.Name, "Helm", utils.FindStatusTextWithColor(statuses.GetHelms(), helm.Id)})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getOrganizationProjectEnvironmentContextResourcesIds(qoveryAPIClient *qovery.APIClient) (string, string, string, error) {
	organizationId, err := getOrganizationContextResourceId(qoveryAPIClient, organizationName)

	if err != nil {
		return "", "", "", err
	}

	projectId, err := getProjectContextResourceId(qoveryAPIClient, projectName, organizationId)

	if err != nil {
		return organizationId, "", "", err
	}

	environmentId, err := getEnvironmentContextResourceId(qoveryAPIClient, environmentName, projectId)

	if err != nil {
		return organizationId, projectId, "", err
	}

	return organizationId, projectId, environmentId, nil
}

func getOrganizationProjectContextResourcesIds(qoveryAPIClient *qovery.APIClient) (string, string, error) {
	organizationId, err := getOrganizationContextResourceId(qoveryAPIClient, organizationName)

	if err != nil {
		return "", "", err
	}

	projectId, err := getProjectContextResourceId(qoveryAPIClient, projectName, organizationId)

	if err != nil {
		return organizationId, "", err
	}

	return organizationId, projectId, nil
}

func getOrganizationContextResourceId(qoveryAPIClient *qovery.APIClient, organizationName string) (string, error) {
	if strings.TrimSpace(organizationName) == "" {
		id, _, err := utils.CurrentOrganization(true)
		if err != nil {
			return "", err
		}

		return string(id), nil
	}

	// find organization by name
	organizations, _, err := qoveryAPIClient.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute()

	if err != nil {
		return "", err
	}

	organization := utils.FindByOrganizationName(organizations.GetResults(), organizationName)
	if organization == nil {
		return "", errors.Errorf("organization %s not found", organizationName)
	}

	return organization.Id, nil
}

func getProjectContextResourceId(qoveryAPIClient *qovery.APIClient, projectName string, organizationId string) (string, error) {
	if strings.TrimSpace(projectName) == "" {
		id, _, err := utils.CurrentProject(true)
		if err != nil {
			return "", err
		}

		return string(id), nil
	}

	if strings.TrimSpace(organizationId) == "" {
		// avoid making a call to the API if the organization id is not set
		return "", nil
	}

	// find project id by name
	projects, _, err := qoveryAPIClient.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()

	if err != nil {
		return "", err
	}

	project := utils.FindByProjectName(projects.GetResults(), projectName)
	if project == nil {
		return "", errors.Errorf("project %s not found", projectName)
	}

	return project.Id, nil
}

func getEnvironmentContextResourceId(qoveryAPIClient *qovery.APIClient, environmentName string, projectId string) (string, error) {
	if strings.TrimSpace(environmentName) == "" {
		id, _, err := utils.CurrentEnvironment(true)
		if err != nil {
			return "", err
		}

		return string(id), nil
	}

	if strings.TrimSpace(projectId) == "" {
		// avoid making a call to the API if the project id is not set
		return "", nil
	}

	// find environment id by name
	environments, _, err := qoveryAPIClient.EnvironmentsAPI.ListEnvironment(context.Background(), projectId).Execute()

	if err != nil {
		return "", err
	}

	environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)
	if environment == nil {
		return "", errors.Errorf("environment %s not found", environmentName)
	}

	return environment.Id, nil
}

func getApplicationContextResource(qoveryAPIClient *qovery.APIClient, applicationName string, environmentId string) (*qovery.Application, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find applications id by name
	applications, _, err := qoveryAPIClient.ApplicationsAPI.ListApplication(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	application := utils.FindByApplicationName(applications.GetResults(), applicationName)

	if application == nil {
		return nil, errors.Errorf("application %s not found", applicationName)
	}

	return application, nil
}

func getContainerContextResource(qoveryAPIClient *qovery.APIClient, containerName string, environmentId string) (*qovery.ContainerResponse, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find containers id by name
	containers, _, err := qoveryAPIClient.ContainersAPI.ListContainer(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	container := utils.FindByContainerName(containers.GetResults(), containerName)

	if container == nil {
		return nil, errors.Errorf("container %s not found", containerName)
	}

	return container, nil
}

func getJobContextResource(qoveryAPIClient *qovery.APIClient, jobName string, environmentId string) (*qovery.JobResponse, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find jobs id by name
	jobs, _, err := qoveryAPIClient.JobsAPI.ListJobs(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	job := utils.FindByJobName(jobs.GetResults(), jobName)

	if job == nil {
		return nil, errors.Errorf("job %s not found", jobName)
	}

	return job, nil
}

func getHelmContextResource(qoveryAPIClient *qovery.APIClient, helmName string, environmentId string) (*qovery.HelmResponse, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find helms id by name
	helms, _, err := qoveryAPIClient.HelmsAPI.ListHelms(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	helm := utils.FindByHelmName(helms.GetResults(), helmName)

	if helm == nil {
		return nil, errors.Errorf("helm %s not found", helmName)
	}

	return helm, nil
}

func getServiceJsonOutput(statuses qovery.EnvironmentStatuses, apps []qovery.Application, containers []qovery.ContainerResponse, jobs []qovery.JobResponse, databases []qovery.Database, helms []qovery.HelmResponse) string {
	var results []interface{}

	for _, app := range apps {
		m := map[string]interface{}{
			"id":     app.Id,
			"name":   app.Name,
			"type":   "application",
			"status": utils.FindStatus(statuses.GetApplications(), app.Id),
		}

		results = append(results, m)
	}

	for _, container := range containers {
		m := map[string]interface{}{
			"id":     container.Id,
			"name":   container.Name,
			"type":   "container",
			"status": utils.FindStatus(statuses.GetContainers(), container.Id),
		}

		results = append(results, m)
	}

	for _, job := range jobs {
		jobType := "lifecycle"
		if job.CronJobResponse != nil {
			jobType = "cronjob"
		}

		m := map[string]interface{}{
			"id":     utils.GetJobId(&job),
			"name":   utils.GetJobName(&job),
			"type":   jobType,
			"status": utils.FindStatus(statuses.GetJobs(), utils.GetJobId(&job)),
		}

		results = append(results, m)
	}

	for _, helm := range helms {
		m := map[string]interface{}{
			"id":     helm.Id,
			"name":   helm.Name,
			"type":   "helm",
			"status": utils.FindStatus(statuses.GetHelms(), helm.Id),
		}

		results = append(results, m)
	}

	for _, db := range databases {
		m := map[string]interface{}{
			"id":     db.Id,
			"name":   db.Name,
			"type":   "database",
			"status": utils.FindStatus(statuses.GetDatabases(), db.Id),
		}

		results = append(results, m)
	}

	j, err := json.Marshal(results)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(j)
}

func getMarkdownOutput(client qovery.APIClient, orgId string, projectId string, envId string, apps []qovery.Application, containers []qovery.ContainerResponse, jobs []qovery.JobResponse, databases []qovery.Database) string {
	env, _, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), envId).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	header := fmt.Sprintf(`[![Qovery Preview](https://www.qovery.com/images/logo-white.svg)](https://www.qovery.com)
---

Here is the [%s](%s) environment services.

Click on the links below to access the different services:
`, env.Name, fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, projectId, envId))

	body := `
| Service | Logs | Preview URL |
|---------|------|-------------|`

	footer := `
---

Powered by [Qovery](https://qovery.com).`

	na := "N/A"
	for _, app := range apps {
		previewUrl := getApplicationPreviewUrl(client, app.Id)
		if previewUrl != nil {
			p := fmt.Sprintf("[Link](%s)", *previewUrl)
			previewUrl = &p
		} else {
			previewUrl = &na
		}

		consoleLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, app.Id)
		consoleLogsLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, app.Id)
		body += fmt.Sprintf("\n| [%s](%s) | [Show logs](%s) | %s |", app.Name, consoleLink, consoleLogsLink, *previewUrl)
	}

	for _, container := range containers {
		previewUrl := getContainerPreviewUrl(client, container.Id)
		if previewUrl != nil {
			p := fmt.Sprintf("[Link](%s)", *previewUrl)
			previewUrl = &p
		} else {
			previewUrl = &na
		}

		consoleLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, container.Id)
		consoleLogsLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, container.Id)
		body += fmt.Sprintf("\n| [%s](%s) | [Show logs](%s) | %s |", container.Name, consoleLink, consoleLogsLink, *previewUrl)
	}

	for _, job := range jobs {
		consoleLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, utils.GetJobId(&job))
		consoleLogsLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, utils.GetJobId(&job))
		body += fmt.Sprintf("\n| [%s](%s) | [Show logs](%s) | %s |", utils.GetJobName(&job), consoleLink, consoleLogsLink, na)
	}

	for _, db := range databases {
		consoleLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/database/%s", orgId, projectId, envId, db.Id)
		consoleLogsLink := fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/deployment-logs", orgId, projectId, envId, db.Id)
		body += fmt.Sprintf("\n| [%s](%s) | [Show logs](%s) | %s |", db.Name, consoleLink, consoleLogsLink, na)
	}

	return header + body + footer
}

func getJiraOutput(client qovery.APIClient, orgId string, projectId string, envId string, apps []qovery.Application, containers []qovery.ContainerResponse, jobs []qovery.JobResponse, databases []qovery.Database) string {
	env, _, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), envId).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	header := fmt.Sprintf(`[Qovery Preview|%s]
---

Here is the [%s|%s] environment services.

Click on the links below to access the different services:
`, fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, projectId, envId), env.Name, fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, projectId, envId))

	body := `
|| Service || Logs || Preview URL ||`

	footer := `
---

Powered by [Qovery|https://qovery.com].`

	na := "N/A"
	for _, app := range apps {
		previewUrl := getApplicationPreviewUrl(client, app.Id)
		if previewUrl != nil {
			p := fmt.Sprintf("[Link|%s]", *previewUrl)
			previewUrl = &p
		} else {
			previewUrl = &na
		}

		consoleLink := fmt.Sprintf("[Console|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, app.Id))
		consoleLogsLink := fmt.Sprintf("[Logs|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, app.Id))
		body += fmt.Sprintf("\n|| [%s|%s] || %s || %s ||", app.Name, consoleLink, consoleLogsLink, *previewUrl)
	}

	for _, container := range containers {
		previewUrl := getContainerPreviewUrl(client, container.Id)
		if previewUrl != nil {
			p := fmt.Sprintf("[Link|%s]", *previewUrl)
			previewUrl = &p
		} else {
			previewUrl = &na
		}

		consoleLink := fmt.Sprintf("[Console|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, container.Id))
		consoleLogsLink := fmt.Sprintf("[Logs|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, container.Id))
		body += fmt.Sprintf("\n|| [%s|%s] || %s || %s ||", container.Name, consoleLink, consoleLogsLink, *previewUrl)
	}

	for _, job := range jobs {
		consoleLink := fmt.Sprintf("[Console|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/application/%s", orgId, projectId, envId, utils.GetJobId(&job)))
		consoleLogsLink := fmt.Sprintf("[Logs|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/live-logs", orgId, projectId, envId, utils.GetJobId(&job)))
		body += fmt.Sprintf("\n|| [%s|%s] || %s || %s ||", utils.GetJobName(&job), consoleLink, consoleLogsLink, na)
	}

	for _, db := range databases {
		consoleLink := fmt.Sprintf("[Console|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/database/%s", orgId, projectId, envId, db.Id))
		consoleLogsLink := fmt.Sprintf("[Logs|%s]", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s/logs/%s/deployment-logs", orgId, projectId, envId, db.Id))
		body += fmt.Sprintf("\n|| [%s|%s] || %s || %s ||", db.Name, consoleLink, consoleLogsLink, na)
	}

	return header + body + footer
}

func getApplicationPreviewUrl(client qovery.APIClient, appId string) *string {
	links, _, err := client.ApplicationMainCallsAPI.ListApplicationLinks(context.Background(), appId).Execute()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	for _, link := range links.GetResults() {
		if link.Url != nil {
			return link.Url
		}
	}

	return nil
}

func getContainerPreviewUrl(client qovery.APIClient, containerId string) *string {
	links, _, err := client.ContainerMainCallsAPI.ListContainerLinks(context.Background(), containerId).Execute()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	for _, link := range links.GetResults() {
		if link.Url != nil {
			return link.Url
		}
	}

	return nil
}

func init() {
	serviceCmd.AddCommand(serviceListCmd)
	serviceListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	serviceListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	serviceListCmd.Flags().BoolVarP(&markdownFlag, "markdown", "", false, "Markdown output")
	serviceListCmd.Flags().BoolVarP(&jiraFlag, "jira", "", false, "Atlassian Jira output")
	serviceListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
