package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var environmentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Environment delete",
	Long: `DELETE turn off an environment and erase all the data. For example:

	qovery environment delete`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}
	},
}

func init() {
	environmentDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentDeleteCmd)
}
