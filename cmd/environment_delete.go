package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an environment",
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

		_, err = client.EnvironmentMainCallsApi.DeleteEnvironment(auth, envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is deleting!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_DELETED, auth, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentDeleteCmd)
	environmentDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}