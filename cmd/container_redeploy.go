package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)

		_, _, err := client.ContainerActionsAPI.DeployContainer(context.Background(), containerList[0].Id).
			ContainerDeployRequest(qovery.ContainerDeployRequest{}).Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to redeploy container(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		WatchContainerDeployment(client, envId, containerList, watchFlag, qovery.STATEENUM_RESTARTED)
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
