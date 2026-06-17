package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update container external secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

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

		err = utils.UpdateServiceExternalSecret(client, utils.Key, utils.Reference, secretManagerAccessId, container.Id, utils.ContainerType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	containerExternalSecretCmd.AddCommand(containerExternalSecretUpdateCmd)
	containerExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	containerExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "New secret manager access name")

	_ = containerExternalSecretUpdateCmd.MarkFlagRequired("key")
	_ = containerExternalSecretUpdateCmd.MarkFlagRequired("container")
}
