package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentStageEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit deployment stage",
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

		stages, _, err := client.DeploymentStageMainCallsApi.ListEnvironmentDeploymentStage(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
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

		_, _, err = client.DeploymentStageMainCallsApi.EditDeploymentStage(context.Background(), stage.GetId()).DeploymentStageRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Stage updated successfully")
	},
}

func init() {
	environmentStageCmd.AddCommand(environmentStageEditCmd)
	environmentStageEditCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageEditCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageEditCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageEditCmd.Flags().StringVarP(&stageName, "name", "n", "", "Stage Name")
	environmentStageEditCmd.Flags().StringVarP(&newStageName, "new-name", "", "", "New Stage Name")
	environmentStageEditCmd.Flags().StringVarP(&stageDescription, "new-description", "", "", "New Stage Description")

	_ = environmentStageEditCmd.MarkFlagRequired("name")
	_ = environmentStageEditCmd.MarkFlagRequired("new-name")
}
