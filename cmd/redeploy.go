package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var redeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploys your application",
	Long:  `REDEPLOY allows you to (re)deploy your application with the last deployed commit`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true)

		project := io.GetProjectByName(ProjectName, OrganizationName)
		environment := io.GetEnvironmentByName(project.Id, BranchName)
		application := io.GetApplicationByName(project.Id, environment.Id, ApplicationName)

		// TODO how many commits to check?
		for _, commit := range io.ListCommits(10) {
			if application.Repository.CommitId == commit.ID().String() {
				projectId := io.GetProjectByName(ProjectName, OrganizationName).Id
				environmentId := io.GetEnvironmentByName(projectId, BranchName).Id
				applicationId := io.GetApplicationByName(projectId, environmentId, ApplicationName).Id
				io.Deploy(projectId, environmentId, applicationId, commit.Hash.String())
				fmt.Println("Redeployed application with commit " + commit.Hash.String())
				return
			}
		}

		fmt.Println("Could not redeploy.")
		fmt.Println("Try to deploy your application from specific commit instead.")
		fmt.Println(" ex: qovery deploy list // displays latest commits")
		fmt.Println("     qovery deploy <commit_id> // deploys application from selected commitId")
	},
}

func init() {
	redeployCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	redeployCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	redeployCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	redeployCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	RootCmd.AddCommand(redeployCmd)
}
