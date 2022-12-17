package cmd

import (
	"context"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"strings"
)

var organizationName string
var projectName string
var environmentName string

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

		appStatuses, _, err := client.ApplicationsApi.GetEnvironmentApplicationStatus(auth, envId).Execute()
		databaseStatuses, _, err := client.DatabasesApi.GetEnvironmentDatabaseStatus(auth, envId).Execute()
		containerStatuses, _, err := client.ContainersApi.GetEnvironmentContainerStatus(auth, envId).Execute()
		jobStatuses, _, err := client.JobsApi.GetEnvironmentJobStatus(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		var data [][]string

		for _, app := range apps.GetResults() {
			data = append(data, []string{app.GetName(), "Application", getStatus(appStatuses.GetResults(), app.Id)})
		}

		for _, container := range containers.GetResults() {
			data = append(data, []string{container.Name, "Container", getStatus(containerStatuses.GetResults(), container.Id)})
		}

		for _, job := range jobs.GetResults() {
			data = append(data, []string{job.Name, "Job", getStatus(jobStatuses.GetResults(), job.Id)})
		}

		for _, database := range databases.GetResults() {
			data = append(data, []string{database.Name, "Database", getStatus(databaseStatuses.GetResults(), database.Id)})
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
		// find organization id by name
		organizations, _, err := qoveryAPIClient.OrganizationMainCallsApi.ListOrganization(auth).Execute()

		if err != nil {
			return "", "", "", err
		}

		for _, o := range organizations.GetResults() {
			if o.Name == organizationName {
				organizationId = o.Id
				break
			}
		}
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

		for _, p := range projects.GetResults() {
			if p.Name == projectName {
				projectId = p.Id
				break
			}
		}
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

		for _, e := range environments.GetResults() {
			if e.Name == environmentName {
				environmentId = e.Id
				break
			}
		}
	}

	return organizationId, projectId, environmentId, nil
}

func getStatus(refObjectStatuses []qovery.ReferenceObjectStatus, serviceId string) string {
	status := "Unknown"

	for _, s := range refObjectStatuses {
		if serviceId == s.Id {
			if s.State == qovery.STATEENUM_RUNNING {
				status = pterm.FgGreen.Sprintf(string(s.State))
			} else if strings.HasSuffix(string(s.State), "ERROR") {
				status = pterm.FgRed.Sprintf(string(s.State))
			} else if strings.HasSuffix(string(s.State), "ING") {
				status = pterm.FgLightBlue.Sprintf(string(s.State))
			} else if strings.HasSuffix(string(s.State), "QUEUED") {
				status = pterm.FgLightYellow.Sprintf(string(s.State))
			} else if s.State == qovery.STATEENUM_READY {
				status = pterm.FgYellow.Sprintf(string(s.State))
			} else {
				status = string(s.State)
			}

			if s.Message != nil && *s.Message != "" {
				status += " (" + *s.Message + ")"
			}

			break
		}
	}

	return status
}

func init() {
	serviceCmd.AddCommand(serviceListCmd)
	serviceListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	serviceListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
