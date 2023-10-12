package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var containerCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a container deployment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
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

		msg, err := utils.CancelServiceDeployment(client, envId, container.Id, utils.ContainerType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		utils.Println(fmt.Sprintf("Container %s deployment cancelled!", pterm.FgBlue.Sprintf(containerName)))
	},
}

func init() {
	containerCmd.AddCommand(containerCancelCmd)
	containerCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerCancelCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cancel until it's done or an error occurs")

	_ = containerCancelCmd.MarkFlagRequired("container")
}
