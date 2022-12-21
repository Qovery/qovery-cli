package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an environment",
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

		_, _, err = client.EnvironmentActionsApi.StopEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Environment is stopping!")

		if watchFlag {
			utils.WatchEnvironment(envId, qovery.STATEENUM_STOPPED, client)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentStopCmd)
	environmentStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
}
