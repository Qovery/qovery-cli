package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/pkg/application"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an application",
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

		var changeAutoDeploy = false
		if cmd.Flags().Changed("auto-deploy") {
			changeAutoDeploy = true
		}

		application.ApplicationUpdate(client, envId, applicationName, applicationBranch, applicationAutoDeploy, changeAutoDeploy)

		utils.Println(fmt.Sprintf("Application %s updated!", pterm.FgBlue.Sprintf("%s", applicationName)))
	},
}

func init() {
	applicationCmd.AddCommand(applicationUpdateCmd)
	applicationUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationUpdateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationUpdateCmd.Flags().StringVarP(&applicationBranch, "branch", "", "", "Application Git Branch")
	applicationUpdateCmd.Flags().BoolVarP(&applicationAutoDeploy, "auto-deploy", "", false, "Application Auto Deploy")

	_ = applicationUpdateCmd.MarkFlagRequired("application")
}
