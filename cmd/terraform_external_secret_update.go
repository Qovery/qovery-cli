package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var terraformExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update terraform external secret",
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

		secretManagerAccessId, err := getSecretManagerAccessIdByName(client, organizationId, envId, utils.SecretManagerAccessName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.UpdateServiceExternalSecret(client, utils.Key, utils.Reference, secretManagerAccessId, terraform.Id, utils.TerraformType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	terraformExternalSecretCmd.AddCommand(terraformExternalSecretUpdateCmd)
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	terraformExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "New secret manager access name")

	_ = terraformExternalSecretUpdateCmd.MarkFlagRequired("key")
	_ = terraformExternalSecretUpdateCmd.MarkFlagRequired("terraform")
}
