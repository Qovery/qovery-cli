package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var environmentEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable to an environment. For example:

	qovery environment env delete`,
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

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		p := api.GetProjectByName(ProjectName)
		e := api.GetEnvironmentByName(p.Id, BranchName)
		ev := api.ListEnvironmentEnvironmentVariables(p.Id, e.Id).GetEnvironmentVariableByKey(args[0])
		api.DeleteEnvironmentEnvironmentVariable(ev.Id, p.Id, e.Id)
		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	environmentEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentEnvCmd.AddCommand(environmentEnvDeleteCmd)
}
