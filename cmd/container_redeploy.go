package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var containerRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a container",
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

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
		}

		_, _, err = client.ContainerActionsApi.RestartContainer(context.Background(), container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println(fmt.Sprintf("Redeploying container %s in progress..", pterm.FgBlue.Sprintf(containerName)))

		if watchFlag {
			utils.WatchContainer(container.Id, envId, client)
		}
	},
}

func init() {
	containerCmd.AddCommand(containerRedeployCmd)
	containerRedeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerRedeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerRedeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerRedeployCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerRedeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")

	_ = containerRedeployCmd.MarkFlagRequired("container")
}
