package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentStageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create deployment stage",
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

		req := qovery.DeploymentStageRequest{
			Name: stageName,
		}

		desc := qovery.NullableString{}
		desc.Set(&stageDescription)

		if stageDescription != "" {
			req.Description = desc
		}

		_, _, err = client.DeploymentStageMainCallsApi.CreateEnvironmentDeploymentStage(context.Background(), environmentId).DeploymentStageRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Stage created successfully")
	},
}

func init() {
	environmentStageCmd.AddCommand(environmentStageCreateCmd)
	environmentStageCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageCreateCmd.Flags().StringVarP(&stageName, "name", "n", "", "Stage Name")
	environmentStageCreateCmd.Flags().StringVarP(&stageDescription, "description", "d", "", "Stage Description")

	_ = environmentStageCreateCmd.MarkFlagRequired("name")
}
