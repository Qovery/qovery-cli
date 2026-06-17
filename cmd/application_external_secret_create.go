package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create application external secret",
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

		secretManagerAccessId, err := getSecretManagerAccessIdByName(client, organizationId, envId, utils.SecretManagerAccessName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateServiceExternalSecret(client, projectId, envId, application.Id, utils.ApplicationScope, utils.Key, utils.Reference, secretManagerAccessId, utils.MountPath)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	applicationExternalSecretCmd.AddCommand(applicationExternalSecretCreateCmd)
	applicationExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "Secret manager access name")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&utils.ApplicationScope, "scope", "", "APPLICATION", "Scope of this external secret <PROJECT|ENVIRONMENT|APPLICATION>")
	applicationExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = applicationExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = applicationExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = applicationExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-name")
	_ = applicationExternalSecretCreateCmd.MarkFlagRequired("application")
}
