package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pterm/pterm"
	"os"
	"slices"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var skipPausedServicesFlag bool

var environmentDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an environment",
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

		if servicesJson != "" && skipPausedServicesFlag {
			utils.PrintlnError(fmt.Errorf("services and skip-paused-services flags are mutually exclusive"))
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

		} else if servicesJson == "" {
			// Deploy the whole env
			_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(context.Background(), envId).Execute()
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
	},
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

	// Gather all non stopped services
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

func init() {
	environmentCmd.AddCommand(environmentDeployCmd)
	environmentDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeployCmd.Flags().StringVarP(&servicesJson, "services", "", "", "Services to deploy (JSON Format: https://api-doc.qovery.com/#tag/Environment-Actions/operation/deployAllServices)")
	environmentDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
	environmentDeployCmd.Flags().BoolVarP(&skipPausedServicesFlag, "skip-paused-services", "", false, "Skip paused services: paused services won't be started / deployed")
}
