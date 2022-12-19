package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel an environment deployment",
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

		_, _, err = client.EnvironmentActionsApi.CancelEnvironmentDeployment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is canceling!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_CANCELED, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentCancelCmd)
	environmentCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
