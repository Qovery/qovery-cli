package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var helmEnvAliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage helm environment variable and secret aliases",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	helmEnvCmd.AddCommand(helmEnvAliasCmd)
}
