package cmd

import (
	"context"
	"errors"
	"os"

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
		var currentStageId string
		for _, stage := range stages.GetResults() {
			service, _ = getServiceByName(client, stage.GetServices(), serviceName)
			if service != nil {
				currentStageId = stage.GetId()
				break
			}
		}

		if service == nil {
			utils.PrintlnError(errors.New("service not found"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := qovery.AttachServiceToDeploymentStageRequest{}
		req.SetIsSkipped(false)

		_, _, err = client.DeploymentStageMainCallsAPI.
			AttachServiceToDeploymentStage(context.Background(), currentStageId, service.GetServiceId()).
			AttachServiceToDeploymentStageRequest(req).
			Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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
