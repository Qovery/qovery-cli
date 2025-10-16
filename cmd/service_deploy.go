package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	serviceDeployName      string
	serviceDeployNames     string
	serviceDeployCommitId  string
	serviceDeployTag       string
	serviceDeployWatchFlag bool
)

var serviceDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a service (application, container, database, job, or helm)",
	Long: `Deploy a service by automatically detecting its type.
This command works with applications, containers, databases, jobs (cronjobs and lifecycle), and helm charts.

Version parameters:
  --commit-id: For applications and git-based jobs/helms
  --tag: For containers and image-based jobs

Examples:
  qovery service deploy -n my-app --commit-id abc123
  qovery service deploy -n my-container --tag v1.2.3
  qovery service deploy -n my-database
  qovery service deploy --services "service1,service2,service3"`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateServiceDeployArguments(serviceDeployName, serviceDeployNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// Get all services to deploy
		servicesToDeploy := getServicesToDeployByNames(client, envId, serviceDeployName, serviceDeployNames)

		if len(servicesToDeploy) == 0 {
			utils.PrintlnError(fmt.Errorf("no services found to deploy"))
			os.Exit(1)
		}

		// Group services by type
		var applications []*qovery.Application
		var containers []*qovery.ContainerResponse
		var databases []*qovery.Database
		var jobs []*qovery.JobResponse
		var helms []*qovery.HelmResponse

		for _, svc := range servicesToDeploy {
			switch svc.Type {
			case utils.ApplicationType:
				applications = append(applications, svc.Application)
			case utils.ContainerType:
				containers = append(containers, svc.Container)
			case utils.DatabaseType:
				databases = append(databases, svc.Database)
			case utils.JobType:
				jobs = append(jobs, svc.Job)
			case utils.HelmType:
				helms = append(helms, svc.Helm)
			}
		}

		// Deploy services
		var err error
		if len(applications) > 0 {
			err = utils.DeployApplications(client, envId, applications, serviceDeployCommitId)
			checkError(err)
		}
		if len(containers) > 0 {
			err = utils.DeployContainers(client, envId, containers, serviceDeployTag)
			checkError(err)
		}
		if len(databases) > 0 {
			err = utils.DeployDatabases(client, envId, databases)
			checkError(err)
		}
		if len(jobs) > 0 {
			err = utils.DeployJobs(client, envId, jobs, serviceDeployCommitId, serviceDeployTag)
			checkError(err)
		}
		if len(helms) > 0 {
			err = utils.DeployHelms(client, envId, helms, "", serviceDeployCommitId, "")
			checkError(err)
		}

		// Print confirmation
		serviceNames := make([]string, len(servicesToDeploy))
		for i, svc := range servicesToDeploy {
			serviceNames[i] = svc.Name
		}
		utils.Println(fmt.Sprintf("Request to deploy service(s) %s has been queued..",
			pterm.FgBlue.Sprintf("%s", strings.Join(serviceNames, ", "))))

		// Watch deployment
		watchServiceDeployment(client, envId, servicesToDeploy, serviceDeployWatchFlag)
	},
}

type serviceDeployInfo struct {
	Name        string
	Type        utils.ServiceType
	Application *qovery.Application
	Container   *qovery.ContainerResponse
	Database    *qovery.Database
	Job         *qovery.JobResponse
	Helm        *qovery.HelmResponse
}

func validateServiceDeployArguments(serviceName string, serviceNames string) {
	if serviceName == "" && serviceNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --service or --services"))
		os.Exit(1)
		panic("unreachable")
	}

	if serviceName != "" && serviceNames != "" {
		utils.PrintlnError(fmt.Errorf("use either --service or --services, not both"))
		os.Exit(1)
		panic("unreachable")
	}
}

func getServicesToDeployByNames(
	client *qovery.APIClient,
	environmentId string,
	serviceName string,
	serviceNames string,
) []serviceDeployInfo {
	var result []serviceDeployInfo

	// Build list of service names to look for
	var namesToFind []string
	if serviceName != "" {
		namesToFind = append(namesToFind, serviceName)
	}
	if serviceNames != "" {
		for _, name := range strings.Split(serviceNames, ",") {
			namesToFind = append(namesToFind, strings.TrimSpace(name))
		}
	}

	// Get all services from the environment
	applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), environmentId).Execute()
	checkError(err)

	containers, _, err := client.ContainersAPI.ListContainer(context.Background(), environmentId).Execute()
	checkError(err)

	databases, _, err := client.DatabasesAPI.ListDatabase(context.Background(), environmentId).Execute()
	checkError(err)

	jobs, _, err := client.JobsAPI.ListJobs(context.Background(), environmentId).Execute()
	checkError(err)

	helms, _, err := client.HelmsAPI.ListHelms(context.Background(), environmentId).Execute()
	checkError(err)

	// Find each service by name
	for _, name := range namesToFind {
		found := false

		// Check applications
		if app := utils.FindByApplicationName(applications.GetResults(), name); app != nil {
			result = append(result, serviceDeployInfo{
				Name:        name,
				Type:        utils.ApplicationType,
				Application: app,
			})
			found = true
			continue
		}

		// Check containers
		if container := utils.FindByContainerName(containers.GetResults(), name); container != nil {
			result = append(result, serviceDeployInfo{
				Name:      name,
				Type:      utils.ContainerType,
				Container: container,
			})
			found = true
			continue
		}

		// Check databases
		if database := utils.FindByDatabaseName(databases.GetResults(), name); database != nil {
			result = append(result, serviceDeployInfo{
				Name:     name,
				Type:     utils.DatabaseType,
				Database: database,
			})
			found = true
			continue
		}

		// Check jobs
		if job := utils.FindByJobName(jobs.GetResults(), name); job != nil {
			result = append(result, serviceDeployInfo{
				Name: name,
				Type: utils.JobType,
				Job:  job,
			})
			found = true
			continue
		}

		// Check helms
		if helm := utils.FindByHelmName(helms.GetResults(), name); helm != nil {
			result = append(result, serviceDeployInfo{
				Name: name,
				Type: utils.HelmType,
				Helm: helm,
			})
			found = true
			continue
		}

		if !found {
			utils.PrintlnError(fmt.Errorf("service '%s' not found", name))
			utils.PrintlnInfo("You can list all services with: qovery service list")
			os.Exit(1)
			panic("unreachable")
		}
	}

	return result
}

func watchServiceDeployment(
	client *qovery.APIClient,
	envId string,
	services []serviceDeployInfo,
	watchFlag bool,
) {
	if !watchFlag {
		return
	}

	time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)

	if len(services) == 1 {
		// Watch single service
		svc := services[0]
		switch svc.Type {
		case utils.ApplicationType:
			utils.WatchApplication(svc.Application.Id, envId, client)
		case utils.ContainerType:
			utils.WatchContainer(svc.Container.Id, envId, client)
		case utils.DatabaseType:
			utils.WatchDatabase(svc.Database.Id, envId, client)
		case utils.JobType:
			jobId := utils.GetJobId(svc.Job)
			utils.WatchJob(jobId, envId, client)
		case utils.HelmType:
			utils.WatchHelm(svc.Helm.Id, envId, client)
		}
	} else {
		// Watch entire environment
		utils.WatchEnvironment(envId, qovery.STATEENUM_DEPLOYED, client)
	}
}

func init() {
	serviceCmd.AddCommand(serviceDeployCmd)
	serviceDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	serviceDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	serviceDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	serviceDeployCmd.Flags().StringVarP(&serviceDeployName, "service", "n", "", "Service Name")
	serviceDeployCmd.Flags().StringVarP(&serviceDeployNames, "services", "", "", "Service Names (comma separated) Example: --services \"svc1,svc2,svc3\"")
	serviceDeployCmd.Flags().StringVarP(&serviceDeployCommitId, "commit-id", "c", "", "Git Commit ID (for applications and git-based jobs/helms)")
	serviceDeployCmd.Flags().StringVarP(&serviceDeployTag, "tag", "t", "", "Image Tag (for containers and image-based jobs)")
	serviceDeployCmd.Flags().BoolVarP(&serviceDeployWatchFlag, "watch", "w", false, "Watch service status until it's ready or an error occurs")
}
