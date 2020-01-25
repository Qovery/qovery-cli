package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"github.com/xeonx/timeago"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	Long: `LIST show all deployable environment. For example:

	qovery deploy list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML := util.CurrentQoveryYML()
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.Name

			if BranchName == "" || ProjectName == "" || ApplicationName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		ShowDeploymentList(ProjectName, BranchName, ApplicationName)
	},
}

func init() {
	deployListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")
	deployListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	deployListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	deployCmd.AddCommand(deployListCmd)
}

func ShowDeploymentList(projectName string, branchName string, applicationName string) {
	output := []string{
		"branch | date | commit id | deployed",
	}

	environments := api.GetBranchByName(api.GetProjectByName(projectName).Id, branchName).Environments
	if len(environments) == 0 {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	var environment api.Environment
	for _, e := range environments {
		if e.Application.Name == applicationName {
			environment = e
		}
	}

	if environment.Id == "" {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	for _, commit := range util.ListCommits(10) {
		config := timeago.English
		config.Max = 30 * timeago.Day

		if environment.CommitId == commit.ID().String() {
			output = append(output, branchName+" | "+config.Format(commit.Committer.When)+" | "+commit.ID().String()+" | "+"✓")
		} else {
			output = append(output, branchName+" | "+config.Format(commit.Committer.When)+" | "+commit.ID().String()+" | "+"𐄂")
		}
	}

	fmt.Println(environment.BranchId)

	fmt.Println(columnize.SimpleFormat(output))
}
