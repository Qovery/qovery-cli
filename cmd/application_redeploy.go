package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy an application",
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

		_, _, err = client.ApplicationActionsApi.RestartApplication(context.Background(), application.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println(fmt.Sprintf("Redeploying application %s in progress..", pterm.FgBlue.Sprintf(applicationName)))

		if watchFlag {
			utils.WatchApplication(application.Id, envId, client)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationRedeployCmd)
	applicationRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationRedeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationRedeployCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")

	_ = applicationRedeployCmd.MarkFlagRequired("application")
}
