package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strconv"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Long: `LIST show all available applications within a project and environment. For example:

	qovery application list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		ShowApplicationList(ProjectName, BranchName)
	},
}

func init() {
	applicationListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	applicationListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	applicationCmd.AddCommand(applicationListCmd)
}

func ShowApplicationList(projectName string, branchName string) {
	output := []string{
		"name | status | databases | brokers | storage",
	}

	applications := api.ListApplications(api.GetProjectByName(projectName).Id, branchName)

	if applications.Results == nil || len(applications.Results) == 0 {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	for _, a := range applications.Results {
		output = append(output, a.Name+" | "+a.Status+" | "+strconv.Itoa(*a.TotalDatabases)+" | "+
			strconv.Itoa(*a.TotalBrokers)+" | "+strconv.Itoa(*a.TotalStorage))
	}

	fmt.Println(columnize.SimpleFormat(output))
}
