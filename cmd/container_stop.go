package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a container",
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

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, container.Id, utils.ContainerType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Container %s stopped!", pterm.FgBlue.Sprintf(containerName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping container %s in progress..", pterm.FgBlue.Sprintf(containerName)))
		}
	},
}

func init() {
	containerCmd.AddCommand(containerStopCmd)
	containerStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerStopCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")

	_ = containerStopCmd.MarkFlagRequired("container")
}
