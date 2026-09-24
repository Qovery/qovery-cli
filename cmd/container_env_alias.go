package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var containerEnvAliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage container environment variable and secret aliases",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	containerEnvCmd.AddCommand(containerEnvAliasCmd)
}
