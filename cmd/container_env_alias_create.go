package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerEnvAliasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create container environment variable or secret alias",
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

		err = utils.CreateServiceAlias(client, projectId, envId, container.Id, utils.ContainerType, utils.Key, utils.Alias, utils.ContainerScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Alias %s has been created", pterm.FgBlue.Sprintf("%s", utils.Alias)))
	},
}

func init() {
	containerEnvAliasCmd.AddCommand(containerEnvAliasCreateCmd)
	containerEnvAliasCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerEnvAliasCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerEnvAliasCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerEnvAliasCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerEnvAliasCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	containerEnvAliasCreateCmd.Flags().StringVarP(&utils.Alias, "alias", "", "", "Environment variable or secret alias")
	containerEnvAliasCreateCmd.Flags().StringVarP(&utils.ContainerScope, "scope", "", "CONTAINER", "Scope of this alias <PROJECT|ENVIRONMENT|CONTAINER>")

	_ = containerEnvAliasCreateCmd.MarkFlagRequired("key")
	_ = containerEnvAliasCreateCmd.MarkFlagRequired("alias")
	_ = containerEnvAliasCreateCmd.MarkFlagRequired("container")
}
