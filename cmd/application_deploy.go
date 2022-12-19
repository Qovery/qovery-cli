package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var applicationDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
		}

		req := qovery.DeployRequest{
			GitCommitId: *application.GitRepository.DeployedCommitId,
		}

		if applicationCommitId != "" {
			req.GitCommitId = applicationCommitId
		}

		_, _, err = client.ApplicationActionsApi.DeployApplication(context.Background(), application.Id).DeployRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Application is deploying!")

		if watchFlag {
			utils.WatchApplication(application.Id, client)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationDeployCmd)
	applicationDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDeployCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")

	_ = applicationDeployCmd.MarkFlagRequired("application")
}
