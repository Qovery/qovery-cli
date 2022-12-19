package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy an environment",
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

		_, _, err = client.EnvironmentActionsApi.RestartEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is redeploying!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_RUNNING, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentRedeployCmd)
	environmentRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
