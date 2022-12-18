package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var containerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an container",
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
			utils.PrintlnError(errors.New(fmt.Sprintf("Container %s not found", containerName)))
			utils.PrintlnInfo(fmt.Sprintf("You can list all containers with: qovery container list"))
			os.Exit(1)
		}

		req := qovery.ContainerDeployRequest{
			ImageTag: container.Tag,
		}

		if containerTag != "" {
			req.ImageTag = containerTag
		}

		_, _, err = client.ContainerActionsApi.DeployContainer(auth, container.Id).ContainerDeployRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println("Container is starting!")

		if watchFlag {
			utils.WatchContainer(container.Id, auth, client)
		}
	},
}

func init() {
	containerCmd.AddCommand(containerStartCmd)
	containerStartCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerStartCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerStartCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerStartCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerStartCmd.Flags().StringVarP(&containerTag, "tag", "t", "", "Container Tag")
	containerStartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")

	_ = containerStartCmd.MarkFlagRequired("container")
}
