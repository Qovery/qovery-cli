package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var environmentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the current environment",
	Long: `DELETE turn off an environment and erase all the data. For example:

	qovery environment delete`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false)

		isConfirmed := io.AskForStringConfirmation(
			false,
			fmt.Sprintf("Type '%s' to delete this environment and erase its associated data", BranchName),
			BranchName)
		if !isConfirmed {
			return
		}

		projectId := io.GetProjectByName(ProjectName, OrganizationName).Id

		io.DeleteEnvironment(projectId, io.GetEnvironmentByName(projectId, BranchName).Id)

		fmt.Println(color.YellowString("deletion in progress..."))
		fmt.Println("Hint: type \"qovery status --watch\" to track the progression of the deletion")
	},
}

func init() {
	environmentDeleteCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentDeleteCmd)
}
