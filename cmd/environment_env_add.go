package cmd

import (
	"fmt"
	"github.com/Qovery/qovery-cli/io"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var environmentEnvAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Add environment variable",
	Long: `ADD an environment variable to an environment. For example:

	qovery environment env add`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false, true)

		if len(args) != 2 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName, OrganizationName)
		e := io.GetEnvironmentByName(p.Id, BranchName, true)

		io.CreateEnvironmentEnvironmentVariable(io.EnvironmentVariable{Key: args[0], Value: args[1]}, p.Id, e.Id)
		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	environmentEnvAddCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentEnvAddCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentEnvAddCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentEnvCmd.AddCommand(environmentEnvAddCmd)
}
