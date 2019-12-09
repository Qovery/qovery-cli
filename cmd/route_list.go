package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var routeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List routes",
	Long: `LIST show all available routes within a project and environment. For example:

	qovery route list`,
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
	routeListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	routeListCmd.PersistentFlags().StringVarP(&EnvironmentName, "environment", "e", "", "Your environment name")

	routeCmd.AddCommand(routeListCmd)
}
