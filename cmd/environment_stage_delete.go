package cmd

import (
	"context"
	"errors"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentStageDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete deployment stage",
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

		_, err = client.DeploymentStageMainCallsApi.DeleteDeploymentStage(context.Background(), stage.GetId()).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Stage deleted successfully")
	},
}

func GetStageByName(stages []qovery.DeploymentStageResponse, stageName string) (*qovery.DeploymentStageResponse, error) {
	for _, stage := range stages {
		if stage.GetName() == stageName {
			return &stage, nil
		}
	}

	return nil, errors.New("stage not found")
}

func init() {
	environmentStageCmd.AddCommand(environmentStageDeleteCmd)
	environmentStageDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStageDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStageDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStageDeleteCmd.Flags().StringVarP(&stageName, "name", "n", "", "Stage Name")

	_ = environmentStageDeleteCmd.MarkFlagRequired("name")

}
