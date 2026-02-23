package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentStageSkipCmd = &cobra.Command{
	Use:   "skip",
	Short: "Skip service from environment-level deployments",
	Long:  "Mark a service as skipped so it is excluded from environment-level bulk deployments while staying in its current stage. Use 'environment stage unskip' to reverse.",
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
		req.SetIsSkipped(true)

		_, _, err = client.DeploymentStageMainCallsAPI.
			AttachServiceToDeploymentStage(context.Background(), currentStageId, service.GetServiceId()).
			AttachServiceToDeploymentStageRequest(req).
			Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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
