package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)
		validateContainerArguments(containerName, containerNames)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		if isDeploymentQueueEnabledForOrganization(organizationId) {
			serviceIds := buildServiceIdsFromContainerNames(client, envId, containerName, containerNames)
			_, err := client.EnvironmentActionsAPI.
				StopSelectedServices(context.Background(), envId).
				EnvironmentServiceIdsAllRequest(qovery.EnvironmentServiceIdsAllRequest{
					ContainerIds: serviceIds,
				}).
				Execute()
			checkError(err)
			utils.Println(fmt.Sprintf("Request to stop container(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			}
			return
		}

		if containerNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, containerName := range strings.Split(containerNames, ",") {
				trimmedContainerName := strings.TrimSpace(containerName)
				serviceIds = append(serviceIds, utils.FindByContainerName(containers.GetResults(), trimmedContainerName).Id)
			}

			// stop multiple services
			_, err = utils.StopServices(client, envId, serviceIds, utils.ContainerType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Stopping containers %s in progress..", pterm.FgBlue.Sprintf("%s", containerNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

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
			utils.Println(fmt.Sprintf("Container %s stopped!", pterm.FgBlue.Sprintf("%s", containerName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping container %s in progress..", pterm.FgBlue.Sprintf("%s", containerName)))
		}
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

func buildServiceIdsFromContainerNames(
	client *qovery.APIClient,
	environmentId string,
	containerName string,
	containerNames string,
) []string {
	containerList := buildContainerListFromContainerNames(client, environmentId, containerName, containerNames)
	serviceIds := make([]string, len(containerList))

	for i, item := range containerList {
		serviceIds[i] = item.Id
	}
	return serviceIds
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
