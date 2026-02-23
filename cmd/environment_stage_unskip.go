package cmd

import (
	"context"
	"errors"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentStageUnskipCmd = &cobra.Command{
	Use:   "unskip",
	Short: "Unskip service from environment-level deployments",
	Long:  "Remove the skipped flag from a service so it is included again in environment-level bulk deployments.",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		utils.CheckError(err)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		utils.CheckError(err)

		stages, _, err := client.DeploymentStageMainCallsAPI.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()
		utils.CheckError(err)

		var service *qovery.DeploymentStageServiceResponse
		var currentStageId string
		for _, stage := range stages.GetResults() {
			service, _ = getServiceByName(client, stage.GetServices(), serviceName)
			if service != nil {
				currentStageId = stage.GetId()
				break
			}
		}

		if service == nil {
			utils.CheckError(errors.New("service not found"))
		}

		req := qovery.AttachServiceToDeploymentStageRequest{}
		req.SetIsSkipped(false)

		_, _, err = client.DeploymentStageMainCallsAPI.
			AttachServiceToDeploymentStage(context.Background(), currentStageId, service.GetServiceId()).
			AttachServiceToDeploymentStageRequest(req).
			Execute()
		utils.CheckError(err)

		utils.Println("Service \"" + serviceName + "\" is no longer skipped from environment-level deployments")
	},
}

func init() {
	environmentStageCmd.AddCommand(environmentStageUnskipCmd)
	environmentStageUnskipCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageUnskipCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageUnskipCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageUnskipCmd.Flags().StringVarP(&serviceName, "name", "n", "", "Service Name")

	_ = environmentStageUnskipCmd.MarkFlagRequired("name")
}
