package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var terraformExternalSecretDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete terraform external secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

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

		err = utils.DeleteServiceVariable(client, terraform.Id, utils.TerraformType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	terraformExternalSecretCmd.AddCommand(terraformExternalSecretDeleteCmd)
	terraformExternalSecretDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformExternalSecretDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformExternalSecretDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformExternalSecretDeleteCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformExternalSecretDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")

	_ = terraformExternalSecretDeleteCmd.MarkFlagRequired("key")
	_ = terraformExternalSecretDeleteCmd.MarkFlagRequired("terraform")
}
