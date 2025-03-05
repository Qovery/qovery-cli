package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

var containerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)
		_, err := client.EnvironmentActionsAPI.
			StopSelectedServices(context.Background(), envId).
			EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
				ContainerIds: utils.Map(containerList, func(container *qovery.ContainerResponse) string {
					return container.Id
				}),
			}).
			Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Request to stop container(s) %s has been queued...", containerName))
		WatchContainerDeployment(client, envId, containerList, watchFlag, qovery.STATEENUM_STOPPED)
	},
}

func buildContainerListFromContainerNames(
	client *qovery.APIClient,
	environmentId string,
	containerName string,
	containerNames string,
) []*qovery.ContainerResponse {
	var containerList []*qovery.ContainerResponse
	containers, _, err := client.ContainersAPI.ListContainer(context.Background(), environmentId).Execute()
	checkError(err)

	if containerName != "" {
		container := utils.FindByContainerName(containers.GetResults(), containerName)
		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		containerList = append(containerList, container)
	}
	if containerNames != "" {
		for _, containerName := range strings.Split(containerNames, ",") {
			trimmedContainerName := strings.TrimSpace(containerName)
			container := utils.FindByContainerName(containers.GetResults(), trimmedContainerName)
			if container == nil {
				utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
				utils.PrintlnInfo("You can list all containers with: qovery container list")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
			containerList = append(containerList, container)
		}
	}

	return containerList
}

func validateContainerArguments(containerName string, containerNames string) {
	if containerName == "" && containerNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --container \"<container name>\" or --containers \"<container1 name>, <container2 name>\" but not both at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if containerName != "" && containerNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --container and --containers at the same time"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func init() {
	containerCmd.AddCommand(containerStopCmd)
	containerStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerStopCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerStopCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Container Names (comma separated) (ex: --containers \"container1,container2\")")
	containerStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")
}
