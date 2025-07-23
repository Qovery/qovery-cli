package cmd

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.ShowHelpIfNoArgs(cmd, args)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// deploy multiple services
		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)
		err := utils.DeployContainers(client, envId, containerList, containerTag)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy container(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		WatchContainerDeployment(client, envId, containerList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func WatchContainerDeployment(
	client *qovery.APIClient,
	envId string,
	containers []*qovery.ContainerResponse,
	watchFlag bool,
	finalServiceState qovery.StateEnum,
) {
	if watchFlag {
		time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
		if len(containers) == 1 {
			utils.WatchContainer(containers[0].Id, envId, client)
		} else {
			utils.WatchEnvironment(envId, finalServiceState, client)
		}
	}
}

func init() {
	containerCmd.AddCommand(containerDeployCmd)
	containerDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDeployCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDeployCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Container Names (comma separated) (ex: --containers \"container1,container2\")")
	containerDeployCmd.Flags().StringVarP(&containerTag, "tag", "t", "", "Container Tag")
	containerDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")
}
