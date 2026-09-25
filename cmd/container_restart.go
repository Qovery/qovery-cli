package cmd

import (
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
		watch := utils.NewServicesWatch(client, envId, containerIds(containerList), watchFlag)
		err := utils.RebootServices(client, envId, qovery.RebootServicesRequest{ContainerIds: containerIds(containerList)})
		checkError(err)
		utils.Println(fmt.Sprintf("Request to restart container(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		watch.Wait(qovery.STATEENUM_RESTARTED)
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
