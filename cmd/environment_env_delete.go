package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var environmentEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable to an environment. For example:

	qovery environment env delete`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false)

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName, OrganizationName)
		e := io.GetEnvironmentByName(p.Id, BranchName)
		ev := io.ListEnvironmentEnvironmentVariables(p.Id, e.Id).GetEnvironmentVariableByKey(args[0])
		io.DeleteEnvironmentEnvironmentVariable(ev.Id, p.Id, e.Id)
		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	environmentEnvDeleteCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentEnvCmd.AddCommand(environmentEnvDeleteCmd)
}
