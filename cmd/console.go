package cmd

import (
	"fmt"
	"github.com/pkg/browser"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

// consoleCmd represents the console command
var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "A brief description of your command",
	// https://console.qovery.com/platform/organization/eaffd2c3-ba35-43a2-a435-cee2e61d8489/projects/b113a830-9d2e-4365-8384-25df06a928af/environments/3015bb4b-71d1-4c0c-a8b7-03af90c6b218/applications/c16e08ad-cccf-41ca-b1a4-3d3ef0611580/summary
	Run: func(cmd *cobra.Command, args []string) {
		organization, _, err := utils.CurrentOrganization()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		project, _, err := utils.CurrentProject()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		environment, _, err := utils.CurrentEnvironment()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		application, _, err := utils.CurrentApplication()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		url := fmt.Sprintf("https://console.qovery.com/platform/organization/%v/projects/%v/environments/%v/applications/%v/summary", organization, project, environment, application)
		utils.PrintlnInfo("Opening " + url)
		err = browser.OpenURL(url)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(consoleCmd)
}
