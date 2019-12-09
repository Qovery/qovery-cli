package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var storageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List storage",
	Long: `LIST show all available storage within a project and environment. For example:

	qovery storage list`,
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
	storageListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	storageListCmd.PersistentFlags().StringVarP(&EnvironmentName, "environment", "e", "", "Your environment name")

	storageCmd.AddCommand(storageListCmd)
}
