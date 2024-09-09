package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var applicationCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel an application deployment",
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

		msg, err := utils.CancelServiceDeployment(client, envId, application.Id, utils.ApplicationType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		utils.Println(fmt.Sprintf("Application %s deployment cancelled!", pterm.FgBlue.Sprintf("%s", applicationName)))
	},
}

func init() {
	applicationCmd.AddCommand(applicationCancelCmd)
	applicationCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationCancelCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cancel until it's done or an error occurs")

	_ = applicationCancelCmd.MarkFlagRequired("application")
}
