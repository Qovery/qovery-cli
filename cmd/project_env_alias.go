package cmd

import (
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvAliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage project variable and secret aliases",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	projectEnvCmd.AddCommand(projectEnvAliasCmd)
}
