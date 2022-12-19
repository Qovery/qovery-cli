package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var containerDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a container",
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

		req := qovery.ContainerDeployRequest{
			ImageTag: container.Tag,
		}

		if containerTag != "" {
			req.ImageTag = containerTag
		}

		_, _, err = client.ContainerActionsApi.DeployContainer(context.Background(), container.Id).ContainerDeployRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Container is deploying!")

		if watchFlag {
			utils.WatchContainer(container.Id, client)
		}
	},
}

func init() {
	containerCmd.AddCommand(containerDeployCmd)
	containerDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDeployCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDeployCmd.Flags().StringVarP(&containerTag, "tag", "t", "", "Container Tag")
	containerDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")

	_ = containerDeployCmd.MarkFlagRequired("container")
}
