package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var organizationName string
var environmentName string

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List services",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		_, envId, err := getContextResourcesId()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

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
			data = append(data, []string{app.GetName(), "Application", getStatus(appStatuses.GetResults(), app.Id), ""})
		}

		for _, database := range databases.GetResults() {
			data = append(data, []string{database.Name, "Database", getStatus(databaseStatuses.GetResults(), database.Id), ""})
		}

		for _, container := range containers.GetResults() {
			data = append(data, []string{container.Name, "Container", getStatus(containerStatuses.GetResults(), container.Id), ""})
		}

		for _, job := range jobs.GetResults() {
			data = append(data, []string{job.Name, "Job", getStatus(jobStatuses.GetResults(), job.Id), ""})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "URL"}, data)

		if err != nil {
			utils.PrintlnError(err)
			return
		}
	},
}

func getStatus(refObjectStatuses []qovery.ReferenceObjectStatus, serviceId string) string {
	status := "Unknown"

	for _, s := range refObjectStatuses {
		if serviceId == s.Id {
			status = string(s.State)

			if s.Message != nil {
				status += " (" + *s.Message + ")"
			}

			break
		}
	}

	return status
}

func getContextResourcesId() (string, string, error) {
	var organizationId string
	var environmentId string

	if organizationName == "" {
		id, _, err := utils.CurrentOrganization()
		if err != nil {
			return "", "", err
		}

		organizationName = string(id)
	}

	if environmentName == "" {
		id, _, err := utils.CurrentEnvironment()
		if err != nil {
			return "", "", err
		}

		environmentId = string(id)
	}

	return organizationId, environmentId, nil
}

func init() {
	serviceCmd.AddCommand(serviceListCmd)
	serviceListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
