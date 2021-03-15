package cmd

import (
	"fmt"
	"github.com/Qovery/qovery-cli/io"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var projectEnvAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Add environment variable",
	Long: `ADD an environment variable to a project. For example:

	qovery project env add`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, false, false, true)

		if len(args) != 2 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName, OrganizationName)
		io.CreateProjectEnvironmentVariable(io.EnvironmentVariable{Key: args[0], Value: args[1]}, p.Id)

		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	projectEnvAddCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	projectEnvAddCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	projectEnvCmd.AddCommand(projectEnvAddCmd)
}
