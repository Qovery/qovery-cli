package cmd

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a container without redeploying it",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)
		_, _, err := client.EnvironmentActionsAPI.
			RebootServices(context.Background(), envId).
			RebootServicesRequest(qovery.RebootServicesRequest{
				ContainerIds: utils.Map(containerList, func(container *qovery.ContainerResponse) string {
					return container.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to restart container(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		WatchContainerDeployment(client, envId, containerList, watchFlag, qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	containerCmd.AddCommand(containerRestartCmd)
	containerRestartCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerRestartCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerRestartCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerRestartCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerRestartCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Container Names (comma separated) Example: --containers \"container1,container2\"")
	containerRestartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")
}
