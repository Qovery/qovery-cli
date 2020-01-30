package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var projectEnvAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Add environment variable",
	Long: `ADD an environment variable to a project. For example:

	qovery project env add`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			ProjectName = util.CurrentQoveryYML().Application.Project

			if ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		if len(args) != 2 {
			_ = cmd.Help()
			return
		}

		p := api.GetProjectByName(ProjectName)
		api.CreateProjectEnvironmentVariable(api.EnvironmentVariable{Key: args[0], Value: args[1]}, p.Id)
		fmt.Println("ok")
	},
}

func init() {
	projectEnvAddCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	projectEnvCmd.AddCommand(projectEnvAddCmd)
}
