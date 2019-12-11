package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
	Long: `LIST show all available databases within a project and environment. For example:

	qovery database list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		// TODO API call
	},
}

func init() {
	databaseListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	databaseListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	databaseCmd.AddCommand(databaseListCmd)
}
