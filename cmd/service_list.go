package cmd

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var organizationName string
var projectName string
var environmentName string
var watchFlag bool

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
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		apps, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		databases, _, err := client.DatabasesApi.ListDatabase(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		jobs, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string

		for _, app := range apps.GetResults() {
			data = append(data, []string{app.GetName(), "Application", utils.GetStatus(statuses.GetApplications(), app.Id)})
		}

		for _, container := range containers.GetResults() {
			data = append(data, []string{container.Name, "Container", utils.GetStatus(statuses.GetContainers(), container.Id)})
		}

		for _, job := range jobs.GetResults() {
			data = append(data, []string{job.Name, "Job", utils.GetStatus(statuses.GetJobs(), job.Id)})
		}

		for _, database := range databases.GetResults() {
			data = append(data, []string{database.Name, "Database", utils.GetStatus(statuses.GetDatabases(), database.Id)})
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
	var organizationId string

	if strings.TrimSpace(organizationName) == "" {
		id, _, err := utils.CurrentOrganization()
		if err != nil {
			return "", err
		}

		return string(id), nil
	}

	// find organization by name
	organizations, _, err := qoveryAPIClient.OrganizationMainCallsApi.ListOrganization(context.Background()).Execute()

	if err != nil {
		return "", err
	}

	organization := utils.FindByOrganizationName(organizations.GetResults(), organizationName)
	if organization != nil {
		organizationId = organization.Id
	}

	return organizationId, nil
}

func getProjectContextResourceId(qoveryAPIClient *qovery.APIClient, projectName string, organizationId string) (string, error) {
	var projectId string

	if strings.TrimSpace(projectName) == "" {
		id, _, err := utils.CurrentProject()
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
	projects, _, err := qoveryAPIClient.ProjectsApi.ListProject(context.Background(), organizationId).Execute()

	if err != nil {
		return "", err
	}

	project := utils.FindByProjectName(projects.GetResults(), projectName)
	if project != nil {
		projectId = project.Id
	}

	return projectId, nil
}

func getEnvironmentContextResourceId(qoveryAPIClient *qovery.APIClient, environmentName string, projectId string) (string, error) {
	var environmentId string

	if strings.TrimSpace(environmentName) == "" {
		id, _, err := utils.CurrentEnvironment()
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
	environments, _, err := qoveryAPIClient.EnvironmentsApi.ListEnvironment(context.Background(), projectId).Execute()

	if err != nil {
		return "", err
	}

	environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)
	if environment != nil {
		environmentId = environment.Id
	}

	return environmentId, nil
}

func getApplicationContextResource(qoveryAPIClient *qovery.APIClient, applicationName string, environmentId string) (*qovery.Application, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find applications id by name
	applications, _, err := qoveryAPIClient.ApplicationsApi.ListApplication(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	application := utils.FindByApplicationName(applications.GetResults(), applicationName)

	if application == nil {
		return nil, errors.New("application not found")
	}

	return application, nil
}

func getContainerContextResource(qoveryAPIClient *qovery.APIClient, containerName string, environmentId string) (*qovery.ContainerResponse, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find containers id by name
	containers, _, err := qoveryAPIClient.ContainersApi.ListContainer(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	container := utils.FindByContainerName(containers.GetResults(), containerName)

	if container == nil {
		return nil, errors.New("container not found")
	}

	return container, nil
}

func getJobContextResource(qoveryAPIClient *qovery.APIClient, jobName string, environmentId string) (*qovery.JobResponse, error) {
	if strings.TrimSpace(environmentId) == "" {
		// avoid making a call to the API if the environment id is not set
		return nil, nil
	}

	// find jobs id by name
	jobs, _, err := qoveryAPIClient.JobsApi.ListJobs(context.Background(), environmentId).Execute()

	if err != nil {
		return nil, err
	}

	job := utils.FindByJobName(jobs.GetResults(), jobName)

	if job == nil {
		return nil, errors.New("job not found")
	}

	return job, nil
}

func init() {
	serviceCmd.AddCommand(serviceListCmd)
	serviceListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	serviceListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
