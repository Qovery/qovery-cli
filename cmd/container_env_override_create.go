package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerEnvOverrideCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Override container environment variable or secret",
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

		err = utils.CreateServiceOverride(client, projectId, envId, container.Id, utils.ContainerType, utils.Key, utils.Value, utils.ContainerScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("%s has been overidden", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	containerEnvOverrideCmd.AddCommand(containerEnvOverrideCreateCmd)
	containerEnvOverrideCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&utils.Value, "value", "", "", "Environment variable or secret value")
	containerEnvOverrideCreateCmd.Flags().StringVarP(&utils.ContainerScope, "scope", "", "CONTAINER", "Scope of this alias <PROJECT|ENVIRONMENT|CONTAINER>")

	_ = containerEnvOverrideCreateCmd.MarkFlagRequired("key")
	_ = containerEnvOverrideCreateCmd.MarkFlagRequired("container")
	_ = containerEnvOverrideCreateCmd.MarkFlagRequired("value")
}
