package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var terraformExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create terraform external secret",
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

		terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		terraform := utils.FindByTerraformName(terraforms.GetResults(), terraformName)

		if terraform == nil {
			utils.PrintlnError(fmt.Errorf("terraform %s not found", terraformName))
			utils.PrintlnInfo("You can list all terraforms with: qovery terraform list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateServiceExternalSecret(client, projectId, envId, terraform.Id, utils.TerraformScope, utils.Key, utils.Reference, utils.SecretManagerAccessId, utils.MountPath)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	terraformExternalSecretCmd.AddCommand(terraformExternalSecretCreateCmd)
	terraformExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessId, "secret-manager-access-id", "", "", "Secret manager access ID")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&utils.TerraformScope, "scope", "", "TERRAFORM", "Scope of this external secret <PROJECT|ENVIRONMENT|TERRAFORM>")
	terraformExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = terraformExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = terraformExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = terraformExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-id")
	_ = terraformExternalSecretCreateCmd.MarkFlagRequired("terraform")
}
