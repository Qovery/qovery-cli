package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.Name
		}

		ShowDeploymentList(ProjectName, BranchName, ApplicationName)
	},
}

func init() {
	deployListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	deployListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	deployListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	deployCmd.AddCommand(deployListCmd)
}

func ShowDeploymentList(projectName string, branchName string, applicationName string) {
	table := util.GetTable()
	table.SetHeader([]string{"branch", "commit date", "commit id", "commit author", "deployed"})

	project := api.GetProjectByName(projectName)
	environment := api.GetEnvironmentByName(api.GetProjectByName(projectName).Id, branchName)
	application := api.GetApplicationByName(project.Id, environment.Id, applicationName)

	if environment.Id == "" {
		table.Append([]string{"", "", "", ""})
		table.Render()
		return
	}

	// TODO param for n last commits
	for _, commit := range util.ListCommits(10) {
		if application.Repository.CommitId == commit.ID().String() {
			table.Append([]string{branchName, commit.Author.When.String(), commit.ID().String(), commit.Author.Name, color.GreenString("✓")})
		} else {
			table.Append([]string{branchName, commit.Author.When.String(), commit.ID().String(), commit.Author.Name, ""})
		}
	}
	table.Render()
}
