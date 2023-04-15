package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"os"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
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
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// wait until service is ready
		for {
			if utils.IsEnvironmentInATerminalState(envId, client) {
				break
			}

			utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
			time.Sleep(5 * time.Second)
		}
		_, _, err = client.EnvironmentActionsApi.StopEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
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
