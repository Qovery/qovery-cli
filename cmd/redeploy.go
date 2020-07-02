package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var redeployCmd = &cobra.Command{
	Use:   "redeploy",
	Short: "Redeploys your application",
	Long:  `REDEPLOY allows you to (re)deploy your application with the last deployed commit`,

	Run: func(cmd *cobra.Command, args []string) {
		qoveryYML, err := io.CurrentQoveryYML()
		if err != nil {
			io.PrintError("No qovery configuration file found")
			os.Exit(1)
		}

		var branchName = io.CurrentBranchName()
		var projectName = qoveryYML.Application.Project
		var applicationName = qoveryYML.Application.GetSanitizeName()

		project := io.GetProjectByName(projectName)
		environment := io.GetEnvironmentByName(project.Id, branchName)
		application := io.GetApplicationByName(project.Id, environment.Id, applicationName)

		// TODO how many commits to check?
		for _, commit := range io.ListCommits(10) {
			if application.Repository.CommitId == commit.ID().String() {
				projectId := io.GetProjectByName(projectName).Id
				environmentId := io.GetEnvironmentByName(projectId, branchName).Id
				applicationId := io.GetApplicationByName(projectId, environmentId, applicationName).Id
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
	RootCmd.AddCommand(redeployCmd)
}
