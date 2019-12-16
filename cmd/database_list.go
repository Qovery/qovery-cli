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

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
	Long: `LIST show all available databases within a project and environment. For example:

	qovery database list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		output := []string{
			"name | status | type | version | endpoint | port | username | password | application",
		}

		// TODO check nil
		services := api.ListDatabases(api.GetProjectByName(ProjectName).Id, BranchName)

		if services.Results == nil || len(services.Results) == 0 {
			fmt.Println(columnize.SimpleFormat(output))
			return
		}

		for _, a := range services.Results {
			applicationName := "none"

			if a.Application != nil {
				applicationName = a.Application.Name
			}

			output = append(output, a.Name+" | "+a.Status+" | "+a.Type+" | "+a.Version+" | "+a.FQDN+" | "+strconv.Itoa(*a.Port)+
				" | "+a.Username+" | "+a.Password+" | "+applicationName)
		}

		fmt.Println(columnize.SimpleFormat(output))
	},
}

func init() {
	databaseListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	databaseListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	databaseCmd.AddCommand(databaseListCmd)
}
