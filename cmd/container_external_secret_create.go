package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create container external secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

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

		secretManagerAccessId, err := getSecretManagerAccessIdByName(client, organizationId, envId, utils.SecretManagerAccessName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateServiceExternalSecret(client, projectId, envId, container.Id, utils.ContainerScope, utils.Key, utils.Reference, secretManagerAccessId, utils.MountPath)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	containerExternalSecretCmd.AddCommand(containerExternalSecretCreateCmd)
	containerExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerExternalSecretCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	containerExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	containerExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "Secret manager access name")
	containerExternalSecretCreateCmd.Flags().StringVarP(&utils.ContainerScope, "scope", "", "CONTAINER", "Scope of this external secret <PROJECT|ENVIRONMENT|CONTAINER>")
	containerExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = containerExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = containerExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = containerExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-name")
	_ = containerExternalSecretCreateCmd.MarkFlagRequired("container")
}
