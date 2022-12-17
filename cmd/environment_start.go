package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		_, _, envId, err := getContextResourcesId(auth, client)

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		_, _, err = client.EnvironmentActionsApi.DeployEnvironment(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		utils.Println("Environment is starting!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_RUNNING, auth, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentStartCmd)
	environmentStartCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStartCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStartCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
