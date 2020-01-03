package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
)

var brokerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List brokers",
	Long: `LIST show all available brokers within a project and environment. For example:

	qovery broker list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(1)
			}
		}

		ShowBrokerList(ProjectName, BranchName)
	},
}

func init() {
	brokerListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	brokerListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	brokerCmd.AddCommand(brokerListCmd)
}

func ShowBrokerList(projectName string, branchName string) {
	output := []string{
		"name | status | type | version | endpoint | port | username | password | application",
	}

	services := api.ListBrokers(api.GetProjectByName(projectName).Id, branchName)

	if services.Results == nil || len(services.Results) == 0 {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	for _, a := range services.Results {
		applicationName := "none"

		if a.Application != nil {
			applicationName = a.Application.Name
		}
		output = append(output, strings.Join([]string{a.Name,
			a.Status,
			a.Type,
			a.Version,
			a.FQDN,
			intPointerValue(a.Port),
			a.Username,
			a.Password,
			applicationName,
		}, " | "))
	}

	fmt.Println(columnize.SimpleFormat(output))
}
