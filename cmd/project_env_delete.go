package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var projectEnvDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete environment variable",
	Long: `DELETE an environment variable to a project. For example:

	qovery project env delete`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, false, false, true)

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName, OrganizationName)
		ev := io.ListProjectEnvironmentVariables(p.Id).GetEnvironmentVariableByKey(args[0])

		io.DeleteProjectEnvironmentVariable(ev.Id, p.Id)

		fmt.Println(color.GreenString("ok"))
	},
}

func init() {
	projectEnvDeleteCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	projectEnvDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	projectEnvCmd.AddCommand(projectEnvDeleteCmd)
}
