package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var lifecycleStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a lifecycle job",
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

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
		}

		_, _, err = client.JobActionsApi.StopJob(context.Background(), lifecycle.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Lifecycle job is stopping!")

		if watchFlag {
			utils.WatchJob(lifecycle.Id, envId, client)
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleStopCmd)
	lifecycleStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleStopCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")

	_ = lifecycleStopCmd.MarkFlagRequired("lifecycle")
}
