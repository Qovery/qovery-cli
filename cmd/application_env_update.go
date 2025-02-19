package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update application environment variable or secret",
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

		err = utils.UpdateServiceVariable(client, utils.Key, utils.Value, application.Id, utils.ApplicationType)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	applicationEnvCmd.AddCommand(applicationEnvUpdateCmd)
	applicationEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationEnvUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationEnvUpdateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	applicationEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")

	_ = applicationEnvUpdateCmd.MarkFlagRequired("key")
	_ = applicationEnvUpdateCmd.MarkFlagRequired("value")
	_ = applicationEnvUpdateCmd.MarkFlagRequired("application")
}
