package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete container environment variable or secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

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

		err = utils.DeleteByKey(client, projectId, envId, container.Id, utils.ContainerType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	containerEnvCmd.AddCommand(containerEnvDeleteCmd)
	containerEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerEnvDeleteCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = containerEnvDeleteCmd.MarkFlagRequired("key")
	_ = containerEnvDeleteCmd.MarkFlagRequired("container")
}
