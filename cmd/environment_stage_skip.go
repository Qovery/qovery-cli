package cmd

import (
	"context"
	"errors"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentStageSkipCmd = &cobra.Command{
	Use:   "skip",
	Short: "Skip service from environment-level deployments",
	Long:  "Mark a service as skipped so it is excluded from environment-level bulk deployments while staying in its current stage. To reverse this, use 'environment stage unskip' or move the service to a different deployment stage, which automatically clears the skipped status.",
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
		req.SetIsSkipped(true)

		_, _, err = client.DeploymentStageMainCallsAPI.
			AttachServiceToDeploymentStage(context.Background(), currentStageId, service.GetServiceId()).
			AttachServiceToDeploymentStageRequest(req).
			Execute()
		utils.CheckError(err)

		utils.Println("Service \"" + serviceName + "\" is now skipped from environment-level deployments")
	},
}

func init() {
	environmentStageCmd.AddCommand(environmentStageSkipCmd)
	environmentStageSkipCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageSkipCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageSkipCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageSkipCmd.Flags().StringVarP(&serviceName, "name", "n", "", "Service Name")

	_ = environmentStageSkipCmd.MarkFlagRequired("name")
}
