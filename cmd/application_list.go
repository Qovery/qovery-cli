package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Long: `LIST show all available applications within a project and environment. For example:

	qovery application list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			EnvironmentName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if EnvironmentName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		// TODO API call
	},
}

func init() {
	applicationListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationListCmd.PersistentFlags().StringVarP(&EnvironmentName, "environment", "e", "", "Your environment name")

	applicationCmd.AddCommand(applicationListCmd)
}
