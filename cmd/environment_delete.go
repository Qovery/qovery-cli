package cmd

import (
	"context"
	"fmt"
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

		if !utils.IsEnvironmentInATerminalState(envId, client) {
			utils.PrintlnError(fmt.Errorf("environment id '%s' is not in a terminal state. The request is not queued and you must wait "+
				"for the end of the current operation to run your command. Try again in a few moment", envId))
			os.Exit(1)
		}

		_, err = client.EnvironmentMainCallsApi.DeleteEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is deleting!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_DELETED, client)
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
