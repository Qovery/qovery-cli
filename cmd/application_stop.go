package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

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

		msg, err := utils.StopService(client, envId, application.Id, utils.ApplicationType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Application %s stopped!", pterm.FgBlue.Sprintf(applicationName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping application %s in progress..", pterm.FgBlue.Sprintf(applicationName)))
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationStopCmd)
	applicationStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationStopCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationStopCmd.Flags().StringVarP(&applicationCommitId, "commit-id", "c", "", "Application Commit ID")
	applicationStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")

	_ = applicationStopCmd.MarkFlagRequired("application")
}
