package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var routerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List routers",
	Long: `LIST show all available routers within a project and environment. For example:

	qovery router list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		// TODO API call
	},
}

func init() {
	routerListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	routerListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	routerCmd.AddCommand(routerListCmd)
}
