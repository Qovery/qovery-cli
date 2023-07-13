package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationEnvOverrideCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Override application environment variable or secret",
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

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

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

		err = utils.CreateOverride(client, projectId, envId, application.Id, utils.ApplicationType, utils.Key, &utils.Value, utils.ApplicationScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("%s has been overidden", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	applicationEnvOverrideCmd.AddCommand(applicationEnvOverrideCreateCmd)
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&utils.Value, "value", "", "", "Environment variable or secret value")
	applicationEnvOverrideCreateCmd.Flags().StringVarP(&utils.ApplicationScope, "scope", "", "APPLICATION", "Scope of this alias <PROJECT|ENVIRONMENT|APPLICATION>")

	_ = applicationEnvOverrideCreateCmd.MarkFlagRequired("key")
	_ = applicationEnvOverrideCreateCmd.MarkFlagRequired("application")
}
