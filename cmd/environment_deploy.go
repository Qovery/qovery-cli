package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		_, _, envId, err := getContextResourcesId(auth, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		_, _, err = client.EnvironmentActionsApi.DeployEnvironment(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is deploying!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_RUNNING, auth, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentDeployCmd)
	environmentDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
