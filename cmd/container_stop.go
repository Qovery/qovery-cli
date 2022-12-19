package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var containerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a container",
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

		containers, _, err := client.ContainersApi.ListContainer(auth, envId).Execute()

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

		_, _, err = client.ContainerActionsApi.StopContainer(auth, container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Container is stopping!")

		if watchFlag {
			utils.WatchContainer(container.Id, auth, client)
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
