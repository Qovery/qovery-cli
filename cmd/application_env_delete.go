package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete application environment variable or secret",
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

		err = utils.DeleteVariable(client, application.Id, utils.ApplicationType, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	applicationEnvCmd.AddCommand(applicationEnvDeleteCmd)
	applicationEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationEnvDeleteCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = applicationEnvDeleteCmd.MarkFlagRequired("key")
	_ = applicationEnvDeleteCmd.MarkFlagRequired("application")
}
