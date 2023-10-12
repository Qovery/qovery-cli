package cmd

import (
	"context"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
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
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.EnvironmentActionsAPI.CancelEnvironmentDeployment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
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
