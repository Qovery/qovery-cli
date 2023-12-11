package cmd

import (
	"context"
	"errors"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentStageMoveCmd = &cobra.Command{
	Use:   "move",
	Short: "Move service into deployment stage",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		stages, _, err := client.DeploymentStageMainCallsAPI.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var service *qovery.DeploymentStageServiceResponse
		for _, stage := range stages.GetResults() {
			service, _ = getServiceByName(client, stage.GetServices(), serviceName)

			if service != nil {
				break
			}
		}

		if service == nil {
			utils.PrintlnError(errors.New("service not found"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		stage, err := GetStageByName(stages.GetResults(), stageName)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := qovery.DeploymentStageRequest{
			Name: newStageName,
		}

		desc := qovery.NullableString{}
		desc.Set(&stageDescription)

		if stageDescription != "" {
			req.Description = desc
		}

		_, _, err = client.DeploymentStageMainCallsAPI.AttachServiceToDeploymentStage(context.Background(), stage.GetId(), service.GetServiceId()).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Application moved into stage \"" + stageName + "\"")
	},
}

func getServiceByName(client *qovery.APIClient, services []qovery.DeploymentStageServiceResponse, name string) (*qovery.DeploymentStageServiceResponse, error) {
	for _, service := range services {
		switch service.GetServiceType() {
		case "APPLICATION":
			application, _, err := client.ApplicationMainCallsAPI.GetApplication(context.Background(), service.GetServiceId()).Execute()
			if err != nil {
				return nil, err
			}

			if application.GetName() == name {
				return &service, nil
			}
		case "DATABASE":
			database, _, err := client.DatabaseMainCallsAPI.GetDatabase(context.Background(), service.GetServiceId()).Execute()
			if err != nil {
				return nil, err
			}

			if database.GetName() == name {
				return &service, nil
			}
		case "CONTAINER":
			container, _, err := client.ContainerMainCallsAPI.GetContainer(context.Background(), service.GetServiceId()).Execute()
			if err != nil {
				return nil, err
			}

			if container.GetName() == name {
				return &service, nil
			}
		case "JOB":
			job, _, err := client.JobMainCallsAPI.GetJob(context.Background(), service.GetServiceId()).Execute()
			if err != nil {
				return nil, err
			}

			if utils.GetJobName(job) == name {
				return &service, nil
			}
		default:
			return nil, errors.New("service type not found")
		}
	}

	return nil, errors.New("service not found")
}

func init() {
	environmentStageCmd.AddCommand(environmentStageMoveCmd)
	environmentStageMoveCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageMoveCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageMoveCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageMoveCmd.Flags().StringVarP(&serviceName, "name", "n", "", "ServiceLevel Name")
	environmentStageMoveCmd.Flags().StringVarP(&stageName, "stage", "s", "", "Target StageLevel Name")

	_ = environmentStageMoveCmd.MarkFlagRequired("name")
	_ = environmentStageMoveCmd.MarkFlagRequired("stage")
}
