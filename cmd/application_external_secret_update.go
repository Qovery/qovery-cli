package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update application external secret",
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

		applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.UpdateServiceExternalSecret(client, utils.Key, utils.Reference, utils.SecretManagerAccessId, application.Id, utils.ApplicationType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	applicationExternalSecretCmd.AddCommand(applicationExternalSecretUpdateCmd)
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	applicationExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessId, "secret-manager-access-id", "", "", "New secret manager access ID")

	_ = applicationExternalSecretUpdateCmd.MarkFlagRequired("key")
	_ = applicationExternalSecretUpdateCmd.MarkFlagRequired("application")
}
