package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerRedeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploy a container",
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

		msg, err := utils.RedeployService(client, envId, container.Id, container.Name, utils.ContainerType, watchFlag)

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
			utils.Println(fmt.Sprintf("Container %s redeployed!", pterm.FgBlue.Sprintf(containerName)))
		} else {
			utils.Println(fmt.Sprintf("Redeploying container %s in progress..", pterm.FgBlue.Sprintf(containerName)))
		}
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
