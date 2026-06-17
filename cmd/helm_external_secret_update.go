package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update helm external secret",
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

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm := utils.FindByHelmName(helms.GetResults(), helmName)

		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		secretManagerAccessId, err := getSecretManagerAccessIdByName(client, organizationId, envId, utils.SecretManagerAccessName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.UpdateServiceExternalSecret(client, utils.Key, utils.Reference, secretManagerAccessId, helm.Id, utils.HelmType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	helmExternalSecretCmd.AddCommand(helmExternalSecretUpdateCmd)
	helmExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	helmExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "New secret manager access name")

	_ = helmExternalSecretUpdateCmd.MarkFlagRequired("key")
	_ = helmExternalSecretUpdateCmd.MarkFlagRequired("helm")
}
