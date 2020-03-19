package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var environmentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the current environment",
	Long: `DELETE turn off an environment and erase all the data. For example:

	qovery environment delete`,

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

		isConfirmed := util.AskForStringConfirmation(
			false,
			fmt.Sprintf("Type '%s' to delete this environment and erase its associated data", BranchName),
			BranchName)
		if !isConfirmed {
			return
		}

		api.DeleteBranch(api.GetProjectByName(ProjectName).Id, BranchName)
		fmt.Println(color.YellowString("deletion in progress..."))
		fmt.Println("Hint: type \"qovery status --watch\" to track the progression of the deletion")
	},
}

func init() {
	environmentDeleteCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentDeleteCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	environmentCmd.AddCommand(environmentDeleteCmd)
}
