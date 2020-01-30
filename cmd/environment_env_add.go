package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var environmentEnvAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Add environment variable",
	Long: `ADD an environment variable to an environment. For example:

	qovery environment env add`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		if len(args) != 2 {
			_ = cmd.Help()
			return
		}

		p := api.GetProjectByName(ProjectName)
		api.CreateEnvironmentEnvironmentVariable(api.EnvironmentVariable{Key: args[0], Value: args[1]}, p.Id, BranchName)
		fmt.Println("ok")
	},
}

func init() {
	environmentEnvAddCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvAddCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentEnvCmd.AddCommand(environmentEnvAddCmd)
}
