package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `LIST show all available environments. For example:

	qovery environment list`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			ProjectName = util.CurrentQoveryYML().Application.Project

			if ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}
	},
}

func init() {
	environmentListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	environmentCmd.AddCommand(environmentListCmd)
}
