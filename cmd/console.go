package cmd

import (
	"fmt"
	"github.com/pkg/browser"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Opens the application in Qovery Console in your browser",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		organization, _, err := utils.CurrentOrganization(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		project, _, err := utils.CurrentProject(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		environment, _, err := utils.CurrentEnvironment(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		service, err := utils.CurrentService(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		url := fmt.Sprintf("https://console.qovery.com/organization/%v/project/%v/environment/%v/%v/%v/general", organization, project, environment, service.Type, service.ID)
		utils.PrintlnInfo("Opening " + url)
		err = browser.OpenURL(url)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(consoleCmd)
}
