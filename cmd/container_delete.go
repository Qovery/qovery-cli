package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)
		_, err := client.EnvironmentActionsAPI.
			DeleteSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ContainerIds: utils.Map(containerList, func(container *qovery.ContainerResponse) string {
					return container.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to delete container(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		WatchContainerDeployment(client, envId, containerList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	containerCmd.AddCommand(containerDeleteCmd)
	containerDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDeleteCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDeleteCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Container Names (comma separated) (ex: --containers \"container1,container2\")")
	containerDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")
}
