package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationEnvAliasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create application environment variable or secret alias",
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


		err = utils.CreateAlias(client, projectId, envId, application.Id, utils.ApplicationType, utils.Key, utils.Alias,  utils.ApplicationScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Alias %s has been created", pterm.FgBlue.Sprintf(utils.Alias)))
	},
}

func init() {
	applicationEnvAliasCmd.AddCommand(applicationEnvAliasCreateCmd)
	applicationEnvAliasCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&utils.Alias, "alias", "", "", "Environment variable or secret alias")
	applicationEnvAliasCreateCmd.Flags().StringVarP(&utils.ApplicationScope, "scope", "", "APPLICATION", "Scope of this alias <PROJECT|ENVIRONMENT|APPLICATION>")

	_ = applicationEnvAliasCreateCmd.MarkFlagRequired("key")
	_ = applicationEnvAliasCreateCmd.MarkFlagRequired("alias")
	_ = applicationEnvAliasCreateCmd.MarkFlagRequired("application")
}
