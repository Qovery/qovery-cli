package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pterm/pterm"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

func EnvironmentDeploy(client *qovery.APIClient, organizationName string, projectName string, environmentName string, newEnvironmentName string, clusterName string, environmentType string, applyDeploymentRule bool, envId string, servicesJson string, applicationNames string, containerNames string, lifecycleNames string, cronjobNames string, helmNames string, skipPausedServicesFlag bool, watchFlag bool) {

	if (servicesJson != "" || applicationNames != "" || containerNames != "" || lifecycleNames != "" ||
		cronjobNames != "" || helmNames != "") && skipPausedServicesFlag {
		utils.PrintlnError(fmt.Errorf("you can't use --skip-paused-services flag with --services, " +
			"--applications, --containers, --lifecycles, --cronjobs or --helms flags"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	// wait until service is ready
	for {
		if utils.IsEnvironmentInATerminalState(envId, client) {
			break
		}

		utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
		time.Sleep(5 * time.Second)
	}

	if servicesJson != "" {
		// convert servicesJson to DeployAllRequest
		var deployAllRequest qovery.DeployAllRequest
		err := json.Unmarshal([]byte(servicesJson), &deployAllRequest)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.EnvironmentActionsAPI.DeployAllServices(context.Background(), envId).DeployAllRequest(deployAllRequest).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		utils.Println("Services are deploying!")
	} else if applicationNames != "" || containerNames != "" || lifecycleNames != "" || cronjobNames != "" || helmNames != "" {
		deploymentRequest := getDeploymentRequestForMultipleServices(client, envId, applicationNames, containerNames, lifecycleNames, cronjobNames, helmNames)
		_, _, err := client.EnvironmentActionsAPI.DeployAllServices(context.Background(), envId).DeployAllRequest(deploymentRequest).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Services are deploying!")
	}

	if skipPausedServicesFlag {
		// Paused services shouldn't be deployed, let's gather services status
		servicesIDsToDeploy, err := getEligibleServices(client, envId, []qovery.StateEnum{qovery.STATEENUM_STOPPED})
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// Deploy the non stopped services from the env
		request := qovery.DeployAllRequest{}
		// Adding services to be deployed
		for _, applicationID := range servicesIDsToDeploy.ApplicationsIDs {
			request.Applications = append(request.Applications, qovery.DeployAllRequestApplicationsInner{ApplicationId: applicationID})
			utils.Println(fmt.Sprintf("Application %s is deploying!", applicationID))
		}
		for _, containerID := range servicesIDsToDeploy.ContainersIDs {
			request.Containers = append(request.Containers, qovery.DeployAllRequestContainersInner{Id: containerID})
			utils.Println(fmt.Sprintf("Container %s is deploying!", containerID))
		}
		for _, helmID := range servicesIDsToDeploy.HelmsIDs {
			request.Helms = append(request.Helms, qovery.DeployAllRequestHelmsInner{Id: &helmID})
			utils.Println(fmt.Sprintf("Helm %s is deploying!", helmID))
		}
		for _, jobID := range servicesIDsToDeploy.JobsIDs {
			request.Jobs = append(request.Jobs, qovery.DeployAllRequestJobsInner{Id: &jobID})
			utils.Println(fmt.Sprintf("Job %s is deploying!", jobID))
		}
		for _, databaseID := range servicesIDsToDeploy.DatabasesIDs {
			request.Databases = append(request.Databases, databaseID)
			utils.Println(fmt.Sprintf("Database %s is deploying!", databaseID))
		}

		_, _, err = client.EnvironmentActionsAPI.DeployAllServices(context.Background(), envId).DeployAllRequest(request).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

	} else if servicesJson == "" && applicationNames == "" && containerNames == "" && lifecycleNames == "" &&
		cronjobNames == "" && helmNames == "" {
		// Deploy the whole env
		_, _, err := client.EnvironmentActionsAPI.DeployEnvironment(context.Background(), envId).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		utils.Println("Environment is deploying!")
	}

	if watchFlag {
		utils.WatchEnvironment(envId, qovery.STATEENUM_DEPLOYED, client)
	}
}

/**
 * Get deployment request for multiple services
 */
func getDeploymentRequestForMultipleServices(
	client *qovery.APIClient,
	envId string,
	applicationNames string,
	containerNames string,
	lifecycleNames string,
	cronjobNames string,
	helmNames string,
) qovery.DeployAllRequest {
	// Deploy the services from the env
	request := qovery.DeployAllRequest{}

	if applicationNames != "" {
		// Adding applications to be deployed
		for _, nameAndVersion := range strings.Split(applicationNames, ",") {
			name, version := splitServiceNameAndVersion(nameAndVersion)

			apps, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			app := utils.FindByApplicationName(apps.GetResults(), name)

			if app == nil {
				utils.PrintlnError(fmt.Errorf("application %s not found", name))
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			request.Applications = append(request.Applications, qovery.DeployAllRequestApplicationsInner{ApplicationId: app.Id, GitCommitId: version})
		}
	}

	if containerNames != "" {
		// Adding containers to be deployed
		for _, nameAndVersion := range strings.Split(containerNames, ",") {
			name, version := splitServiceNameAndVersion(nameAndVersion)

			containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			container := utils.FindByContainerName(containers.GetResults(), name)

			request.Containers = append(request.Containers, qovery.DeployAllRequestContainersInner{Id: container.Id, ImageTag: version})
		}
	}

	if lifecycleNames != "" || cronjobNames != "" {
		jobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleNames != "" {
			// Adding lifecycle to be deployed
			for _, nameAndVersion := range strings.Split(lifecycleNames, ",") {
				name, version := splitServiceNameAndVersion(nameAndVersion)
				job, gitCommitId, imageTag := getLifecycleJobGitCommitAndImageTag(jobs.GetResults(), name)

				if job == nil {
					utils.PrintlnError(fmt.Errorf("lifecycle %s not found", name))
					os.Exit(1)
					panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
				}

				req := qovery.DeployAllRequestJobsInner{Id: &job.LifecycleJobResponse.Id}
				if gitCommitId != nil {
					req.GitCommitId = version
				} else if imageTag != nil {
					req.ImageTag = version
				}

				request.Jobs = append(request.Jobs, req)
			}
		}

		if cronjobNames != "" {
			// Adding cronjobs to be deployed
			for _, nameAndVersion := range strings.Split(cronjobNames, ",") {
				name, version := splitServiceNameAndVersion(nameAndVersion)

				job, gitCommitId, imageTag := getCronjobGitCommitAndImageTag(jobs.GetResults(), name)

				if job == nil {
					utils.PrintlnError(fmt.Errorf("cronjob %s not found", name))
					os.Exit(1)
					panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
				}

				req := qovery.DeployAllRequestJobsInner{Id: &job.CronJobResponse.Id}
				if gitCommitId != nil {
					req.GitCommitId = version
				} else if imageTag != nil {
					req.ImageTag = version
				}

				request.Jobs = append(request.Jobs, req)
			}
		}
	}

	if helmNames != "" {
		// Adding helms to be deployed
		for _, nameAndVersion := range strings.Split(helmNames, ",") {
			name, version := splitServiceNameAndVersion(nameAndVersion)

			helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			helm := utils.FindByHelmName(helms.GetResults(), name)

			if helm == nil {
				utils.PrintlnError(fmt.Errorf("helm %s not found", name))
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			gitCommitId, chartVersion := getHelmCommitAndChartVersion(client, name)

			req := qovery.DeployAllRequestHelmsInner{Id: &helm.Id}
			if gitCommitId != nil {
				req.GitCommitId = version
			} else if chartVersion != nil {
				req.ChartVersion = version
			}

			request.Helms = append(request.Helms, req)
		}
	}

	return request
}

type Services struct {
	ApplicationsIDs []string
	ContainersIDs   []string
	HelmsIDs        []string
	JobsIDs         []string
	DatabasesIDs    []string
}

func getEligibleServices(client *qovery.APIClient, envId string, servicesStatusesToExclude []qovery.StateEnum) (Services, error) {
	nonStoppedServices := Services{
		ApplicationsIDs: make([]string, 0),
		ContainersIDs:   make([]string, 0),
		HelmsIDs:        make([]string, 0),
		JobsIDs:         make([]string, 0),
		DatabasesIDs:    make([]string, 0),
	}
	envStatuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()
	if err != nil {
		return nonStoppedServices, err
	}

	// Gather all non-stopped services
	for _, serviceStatus := range envStatuses.Applications {
		if !slices.Contains(servicesStatusesToExclude, serviceStatus.GetState()) {
			nonStoppedServices.ApplicationsIDs = append(nonStoppedServices.ApplicationsIDs, serviceStatus.Id)
		}
	}
	for _, serviceStatus := range envStatuses.Containers {
		if !slices.Contains(servicesStatusesToExclude, serviceStatus.GetState()) {
			nonStoppedServices.ContainersIDs = append(nonStoppedServices.ContainersIDs, serviceStatus.Id)
		}
	}
	for _, serviceStatus := range envStatuses.Helms {
		if !slices.Contains(servicesStatusesToExclude, serviceStatus.GetState()) {
			nonStoppedServices.HelmsIDs = append(nonStoppedServices.HelmsIDs, serviceStatus.Id)
		}
	}
	for _, serviceStatus := range envStatuses.Jobs {
		if !slices.Contains(servicesStatusesToExclude, serviceStatus.GetState()) {
			nonStoppedServices.JobsIDs = append(nonStoppedServices.JobsIDs, serviceStatus.Id)
		}
	}
	for _, serviceStatus := range envStatuses.Databases {
		if !slices.Contains(servicesStatusesToExclude, serviceStatus.GetState()) {
			nonStoppedServices.DatabasesIDs = append(nonStoppedServices.DatabasesIDs, serviceStatus.Id)
		}
	}

	return nonStoppedServices, nil
}

/**
 * Split service name and version (if provided)
 */
func splitServiceNameAndVersion(service string) (string, *string) {
	split := strings.Split(service, ":")
	if len(split) == 1 {
		return split[0], nil
	}

	return split[0], &split[1]
}

func getLifecycleJobGitCommitAndImageTag(jobs []qovery.JobResponse, jobName string) (*qovery.JobResponse, *string, *string) {
	var commitId, imageTag *string

	job := utils.FindByJobName(jobs, jobName)

	if job == nil {
		return nil, nil, nil
	}

	if job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf != nil {
		// image tag
		image := job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf.GetImage()
		tag := image.GetTag()
		imageTag = &tag
	} else if job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1 != nil {
		// commit id
		docker := job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1.GetDocker()
		commitId = docker.GitRepository.DeployedCommitId
	}

	return job, commitId, imageTag
}

func getCronjobGitCommitAndImageTag(jobs []qovery.JobResponse, jobName string) (*qovery.JobResponse, *string, *string) {
	var commitId, imageTag *string

	job := utils.FindByJobName(jobs, jobName)

	if job == nil {
		return nil, nil, nil
	}

	if job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf != nil {
		// image tag
		image := job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf.GetImage()
		tag := image.GetTag()
		imageTag = &tag
	} else if job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf1 != nil {
		// commit id
		docker := job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf1.GetDocker()
		commitId = docker.GitRepository.DeployedCommitId
	}

	return job, commitId, imageTag
}

func getHelmCommitAndChartVersion(client *qovery.APIClient, helmId string) (*string, *string) {
	var commitId, chartVersion *string

	// check if the helm version is a chart version or a commit id
	helm, _, err := client.HelmMainCallsAPI.GetHelm(context.Background(), helmId).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	if helm.Source.HelmResponseAllOfSourceOneOf != nil {
		// chart version
		git := helm.Source.HelmResponseAllOfSourceOneOf.GetGit()
		commitId = git.GitRepository.DeployedCommitId
	} else if helm.Source.HelmResponseAllOfSourceOneOf1 != nil {
		// commit id
		git := helm.Source.HelmResponseAllOfSourceOneOf1.GetRepository()
		chartVersion = &git.ChartVersion
	}

	return commitId, chartVersion
}
