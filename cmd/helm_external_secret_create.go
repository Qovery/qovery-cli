package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create helm external secret",
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

		err = utils.CreateServiceExternalSecret(client, projectId, envId, helm.Id, utils.HelmScope, utils.Key, utils.Reference, secretManagerAccessId, utils.MountPath)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	helmExternalSecretCmd.AddCommand(helmExternalSecretCreateCmd)
	helmExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmExternalSecretCreateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	helmExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	helmExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "Secret manager access name")
	helmExternalSecretCreateCmd.Flags().StringVarP(&utils.HelmScope, "scope", "", "HELM", "Scope of this external secret <PROJECT|ENVIRONMENT|HELM>")
	helmExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = helmExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = helmExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = helmExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-name")
	_ = helmExternalSecretCreateCmd.MarkFlagRequired("helm")
}
