package cmd

import (
	"github.com/spf13/cobra"
)

var routerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List routers",
	Long: `LIST show all available routers within a project and environment. For example:

	qovery router list`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false, true)

		// TODO API call
	},
}

func init() {
	routerListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	routerListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	routerListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	routerCmd.AddCommand(routerListCmd)
}
