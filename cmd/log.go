package cmd

import (
	"context"
	"errors"
	_ "fmt"
	"os"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	rawFormat      bool
	logJobName     string
	logServiceName string
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Print your application logs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		getLogs()
	},
}

func getLogs() string {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	client := utils.GetQoveryClient(tokenType, token)

	var service *utils.Service

	orgID, projectID, envID, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	switch {
	case applicationName != "":
		app, err := getApplicationContextResource(client, applicationName, envID)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		service = &utils.Service{ID: utils.Id(app.Id), Name: utils.Name(app.Name), Type: utils.ApplicationType}
	case containerName != "":
		container, err := getContainerContextResource(client, containerName, envID)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		service = &utils.Service{ID: utils.Id(container.Id), Name: utils.Name(container.Name), Type: utils.ContainerType}
	case databaseName != "":
		db, err := getDatabaseContextResource(client, databaseName, envID)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		service = &utils.Service{ID: utils.Id(db.Id), Name: utils.Name(db.Name), Type: utils.DatabaseType}
	case logJobName != "":
		job, err := getJobContextResource(client, logJobName, envID)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		if job.CronJobResponse != nil {
			service = &utils.Service{ID: utils.Id(job.CronJobResponse.Id), Name: utils.Name(job.CronJobResponse.Name), Type: utils.JobType}
		} else if job.LifecycleJobResponse != nil {
			service = &utils.Service{ID: utils.Id(job.LifecycleJobResponse.Id), Name: utils.Name(job.LifecycleJobResponse.Name), Type: utils.JobType}
		}
	case logServiceName != "":
		svc, err := getServiceContextResourceId(client, logServiceName, envID)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		service = svc
	default:
		service, err = utils.CurrentService(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
	}

	e, res, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), envID).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	if res.StatusCode >= 400 {
		utils.PrintlnError(errors.New("Received " + res.Status + " response while fetching environment. "))
		os.Exit(1)
	}

	req := pkg.LogRequest{
		ServiceID:      service.ID,
		OrganizationID: utils.Id(orgID),
		ProjectID:      utils.Id(projectID),
		EnvironmentID:  utils.Id(envID),
		ClusterID:      utils.Id(e.ClusterId),
		RawFormat:      rawFormat,
	}

	pkg.ExecLog(&req)

	// return logRows
	return ""
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVarP(&rawFormat, "raw", "r", false, "display logs in raw format (json)")
	logCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	logCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	logCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	logCmd.Flags().StringVarP(&applicationName, "application", "a", "", "Application Name")
	logCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	logCmd.Flags().StringVarP(&databaseName, "database", "d", "", "Database Name")
	logCmd.Flags().StringVarP(&logJobName, "job", "j", "", "Job Name")
	logCmd.Flags().StringVarP(&logServiceName, "service", "s", "", "Service Name")
}
