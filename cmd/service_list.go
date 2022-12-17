package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"strings"
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

		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		_, _, envId, err := getContextResourcesId(auth, client)

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		apps, _, err := client.ApplicationsApi.ListApplication(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		databases, _, err := client.DatabasesApi.ListDatabase(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		containers, _, err := client.ContainersApi.ListContainer(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		jobs, _, err := client.JobsApi.ListJobs(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
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
			return
		}
	},
}

func getContextResourcesId(auth context.Context, qoveryAPIClient *qovery.APIClient) (string, string, string, error) {
	var organizationId string
	var projectId string
	var environmentId string

	if strings.TrimSpace(organizationName) == "" {
		id, _, err := utils.CurrentOrganization()
		if err != nil {
			return "", "", "", err
		}

		organizationId = string(id)
	} else {
		organizations, _, err := qoveryAPIClient.OrganizationMainCallsApi.ListOrganization(auth).Execute()

		if err != nil {
			return "", "", "", err
		}

		organizationId = utils.FindByOrganizationName(organizations.GetResults(), organizationName).Id
	}

	if strings.TrimSpace(projectName) == "" {
		id, _, err := utils.CurrentProject()
		if err != nil {
			return "", "", "", err
		}

		projectId = string(id)
	} else {
		// find project id by name
		projects, _, err := qoveryAPIClient.ProjectsApi.ListProject(auth, organizationId).Execute()

		if err != nil {
			return "", "", "", err
		}

		projectId = utils.FindByProjectName(projects.GetResults(), organizationName).Id
	}

	if strings.TrimSpace(environmentName) == "" {
		id, _, err := utils.CurrentEnvironment()
		if err != nil {
			return "", "", "", err
		}

		environmentId = string(id)
	} else {
		// find environment id by name
		environments, _, err := qoveryAPIClient.EnvironmentsApi.ListEnvironment(auth, projectId).Execute()

		if err != nil {
			return "", "", "", err
		}

		environmentId = utils.FindByEnvironmentName(environments.GetResults(), environmentName).Id
	}

	return organizationId, projectId, environmentId, nil
}

func init() {
	serviceCmd.AddCommand(serviceListCmd)
	serviceListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	serviceListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
