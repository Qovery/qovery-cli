package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create container environment variable or secret",
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

		err = utils.CreateEnvironmentVariable(client, projectId, envId, container.Id, utils.ContainerScope, utils.Key, utils.Value, utils.IsSecret)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been created", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	containerEnvCmd.AddCommand(containerEnvCreateCmd)
	containerEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerEnvCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerEnvCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	containerEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	containerEnvCreateCmd.Flags().StringVarP(&utils.ContainerScope, "scope", "", "CONTAINER", "Scope of this env var <PROJECT|ENVIRONMENT|CONTAINER>")
	containerEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This environment variable is a secret")

	_ = containerEnvCreateCmd.MarkFlagRequired("key")
	_ = containerEnvCreateCmd.MarkFlagRequired("value")
	_ = containerEnvCreateCmd.MarkFlagRequired("container")
}
