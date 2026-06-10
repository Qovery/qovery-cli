package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationExternalSecretDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete application external secret",
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

		err = utils.DeleteServiceVariable(client, application.Id, utils.ApplicationType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	applicationExternalSecretCmd.AddCommand(applicationExternalSecretDeleteCmd)
	applicationExternalSecretDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationExternalSecretDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationExternalSecretDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationExternalSecretDeleteCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationExternalSecretDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")

	_ = applicationExternalSecretDeleteCmd.MarkFlagRequired("key")
	_ = applicationExternalSecretDeleteCmd.MarkFlagRequired("application")
}
