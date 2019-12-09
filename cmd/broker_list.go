package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var brokerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List brokers",
	Long: `LIST show all available brokers within a project and environment. For example:

	qovery broker list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			EnvironmentName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if EnvironmentName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		// TODO API call
	},
}

func init() {
	brokerListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	brokerListCmd.PersistentFlags().StringVarP(&EnvironmentName, "environment", "e", "", "Your environment name")

	brokerCmd.AddCommand(brokerListCmd)
}
